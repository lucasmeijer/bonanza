package analysis

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"

	"go.starlark.net/starlark"
)

type createInitialConfigurationEnvironment[TReference, TMetadata any] interface {
	model_core.ObjectCapturer[TReference, TMetadata]
	labelResolverEnvironment[TReference]
	getExpectedTransitionOutputEnvironment[TReference]
}

func (c *baseComputer[TReference, TMetadata]) createInitialConfiguration(
	ctx context.Context,
	e createInitialConfigurationEnvironment[TReference, TMetadata],
	thread *starlark.Thread,
	rootPackage label.CanonicalPackage,
	targetPlatform string,
) (model_core.PatchedMessage[*model_core_pb.Reference, TMetadata], error) {
	commandLineOptionPlatformsLabelStr := commandLineOptionPlatformsLabel.String()
	platformExpectedTransitionOutput, err := getExpectedTransitionOutput[TReference, TMetadata](e, rootPackage, commandLineOptionPlatformsLabelStr)
	if err != nil {
		return model_core.PatchedMessage[*model_core_pb.Reference, TMetadata]{}, err
	}

	apparentTargetPlatform, err := label.NewApparentLabel(targetPlatform)
	if err != nil {
		return model_core.PatchedMessage[*model_core_pb.Reference, TMetadata]{}, fmt.Errorf("invalid target platform %#v: %w", targetPlatform, err)
	}
	canonicalTargetPlatform, err := label.Canonicalize(
		newLabelResolver(e),
		rootPackage.GetCanonicalRepo(),
		apparentTargetPlatform,
	)
	if err != nil {
		return model_core.PatchedMessage[*model_core_pb.Reference, TMetadata]{}, fmt.Errorf("failed to resolve target platform %#v: %w", targetPlatform, err)
	}

	return c.applyTransition(
		ctx,
		e,
		model_core.NewSimpleMessage[TReference]((*model_core_pb.Reference)(nil)),
		[]expectedTransitionOutput[TReference]{platformExpectedTransitionOutput},
		thread,
		starlark.StringDict{
			commandLineOptionPlatformsLabelStr: model_starlark.NewLabel[TReference, TMetadata](canonicalTargetPlatform.AsResolved()),
		},
		c.getValueEncodingOptions(e, nil),
	)
}

func (c *baseComputer[TReference, TMetadata]) ComputeExecTransitionValue(ctx context.Context, key model_core.Message[*model_analysis_pb.ExecTransition_Key, TReference], e ExecTransitionEnvironment[TReference, TMetadata]) (PatchedExecTransitionValue, error) {
	allBuiltinsModulesNames := e.GetBuiltinsModuleNamesValue(&model_analysis_pb.BuiltinsModuleNames_Key{})
	rootModuleValue := e.GetRootModuleValue(&model_analysis_pb.RootModule_Key{})
	if !allBuiltinsModulesNames.IsSet() || !rootModuleValue.IsSet() {
		return PatchedExecTransitionValue{}, evaluation.ErrMissingDependency
	}

	// Obtain the identifier of the Starlark transition that is used
	// for exec transitions.
	rootModuleName, err := label.NewModule(rootModuleValue.Message.RootModuleName)
	if err != nil {
		return PatchedExecTransitionValue{}, fmt.Errorf("invalid root module name: %w", err)
	}
	rootPackage := rootModuleName.ToModuleInstance(nil).GetBareCanonicalRepo().GetRootPackage()
	inputConfigurationReference := model_core.Nested(key, key.Message.InputConfigurationReference)
	execConfigCommandLineOption := "@@bazel_tools+//command_line_option:experimental_exec_config"
	execConfigValue, err := c.getBuildSettingValue(
		ctx,
		e,
		rootPackage,
		execConfigCommandLineOption,
		inputConfigurationReference,
	)
	if err != nil {
		return PatchedExecTransitionValue{}, err
	}
	execConfigStr, ok := execConfigValue.Message.Kind.(*model_starlark_pb.Value_Str)
	if !ok {
		return PatchedExecTransitionValue{}, fmt.Errorf("value of build setting %s is not a string", execConfigCommandLineOption)
	}

	// Invoke the exec transition.
	thread := c.newStarlarkThread(ctx, e, allBuiltinsModulesNames.Message.BuiltinsModuleNames)
	expectedOutputs, outputsDict, err := c.performUserDefinedTransition(
		ctx,
		e,
		thread,
		execConfigStr.Str,
		inputConfigurationReference,
		stubbedTransitionAttr{},
	)
	if err != nil {
		return PatchedExecTransitionValue{}, err
	}
	if l := len(outputsDict); l > 1 {
		return PatchedExecTransitionValue{}, fmt.Errorf("exec transition yielded %d configurations, while exactly 1 was expected", l)
	}
	outputs := slices.Collect(maps.Values(outputsDict))[0]

	// Create a new configuration that only contains --platforms set
	// to the platform used for execution.
	patchedInitialOutputConfigurationReference, err := c.createInitialConfiguration(
		ctx,
		e,
		thread,
		rootPackage,
		key.Message.PlatformLabel,
	)
	if err != nil {
		return PatchedExecTransitionValue{}, err
	}

	// Extend the new configuration to include any build settings
	// yielded by the exec transition.
	outputConfigurationReference, err := c.applyTransition(
		ctx,
		e,
		model_core.Unpatch(e, patchedInitialOutputConfigurationReference),
		expectedOutputs,
		thread,
		outputs,
		c.getValueEncodingOptions(e, nil),
	)
	if err != nil {
		return PatchedExecTransitionValue{}, err
	}

	return model_core.NewPatchedMessage(
		&model_analysis_pb.ExecTransition_Value{
			OutputConfigurationReference: outputConfigurationReference.Message,
		},
		model_core.MapReferenceMetadataToWalkers(outputConfigurationReference.Patcher),
	), nil
}
