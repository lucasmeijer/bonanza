package analysis

import (
	"bytes"
	"context"
	"errors"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"

	"google.golang.org/protobuf/types/known/durationpb"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionResult_Key, TReference], e TargetActionResultEnvironment[TReference, TMetadata]) (PatchedTargetActionResultValue, error) {
	commandEncoder, gotCommandEncoder := e.GetCommandEncoderObjectValue(&model_analysis_pb.CommandEncoderObject_Key{})
	patchedConfigurationReference := model_core.Patch(e, model_core.Nested(key, key.Message.ConfigurationReference))
	configuredTarget := e.GetConfiguredTargetValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ConfiguredTarget_Key{
				Label:                  key.Message.Label,
				ConfigurationReference: patchedConfigurationReference.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
		),
	)
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryCreationParametersMessage := e.GetDirectoryCreationParametersValue(&model_analysis_pb.DirectoryCreationParameters_Key{})
	fileCreationParametersMessage := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !gotCommandEncoder ||
		!configuredTarget.IsSet() ||
		!gotDirectoryCreationParameters ||
		!directoryCreationParametersMessage.IsSet() ||
		!fileCreationParametersMessage.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	// Look up the action within the configured target.
	// TODO: Should this be moved into a separate function, so that
	// changes to a single action does not require others to be
	// recomputed?
	actionID := key.Message.ActionId
	action, err := btree.Find(
		ctx,
		c.configuredTargetActionReader,
		model_core.Nested(configuredTarget, configuredTarget.Message.Actions),
		func(entry *model_analysis_pb.ConfiguredTarget_Value_Action) (int, *model_core_pb.Reference) {
			switch level := entry.Level.(type) {
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_:
				return bytes.Compare(actionID, level.Leaf.Id), nil
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Parent_:
				return bytes.Compare(actionID, level.Parent.FirstId), level.Parent.Reference
			default:
				return 0, nil
			}
		},
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}
	if !action.IsSet() {
		return PatchedTargetActionResultValue{}, errors.New("target does not yield an action with the provided identifier")
	}
	actionLevel, ok := action.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_)
	if !ok {
		return PatchedTargetActionResultValue{}, errors.New("action is not a leaf")
	}
	actionLeaf := actionLevel.Leaf

	// Construct the command of the action.
	referenceFormat := c.getReferenceFormat()
	createdCommand, err := model_core.MarshalAndEncodePatchedMessage(
		model_core.NewSimplePatchedMessage[TMetadata](
			&model_command_pb.Command{
				// Arguments: arguments,
				// EnvironmentVariables: enironmentVariables,
				DirectoryCreationParameters: directoryCreationParametersMessage.Message.DirectoryCreationParameters,
				FileCreationParameters:      fileCreationParametersMessage.Message.FileCreationParameters,
				// OutputPathPattern: outputPathPattern,
				WorkingDirectory: (*path.Trace)(nil).GetUNIXString(),
			},
		),
		referenceFormat,
		commandEncoder,
	)

	// Construct the input root of the action.
	rootDirectory, err := getRootForFiles(e, model_core.Nested(action, actionLeaf.Inputs))
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}
	createdRootDirectory, err := model_core.MarshalAndEncodePatchedMessage(
		rootDirectory,
		referenceFormat,
		directoryCreationParameters.GetEncoder(),
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	actionResult := e.GetActionResultValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ActionResult_Key{
				CommandReference: patcher.AddReference(
					createdCommand.Contents.GetReference(),
					e.CaptureCreatedObject(createdCommand),
				),
				// TODO: Should we make the execution
				// timeout on build actions configurable?
				// Bazel with REv2 does not set this field
				// for build actions, relying on the cluster
				// to pick a default.
				ExecutionTimeout:   &durationpb.Duration{Seconds: 3600},
				ExitCodeMustBeZero: true,
				InputRootReference: patcher.AddReference(
					createdRootDirectory.Contents.GetReference(),
					e.CaptureCreatedObject(createdRootDirectory),
				),
				PlatformPkixPublicKey: actionLeaf.PlatformPkixPublicKey,
			},
			model_core.MapReferenceMetadataToWalkers(patcher),
		),
	)
	if !actionResult.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	return PatchedTargetActionResultValue{}, errors.New("TODO: Invoke action!")
}
