package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"

	"google.golang.org/protobuf/proto"

	"go.starlark.net/starlark"
)

func (c *baseComputer[TReference, TMetadata]) ComputeEmptyDefaultInfoValue(ctx context.Context, key *model_analysis_pb.EmptyDefaultInfo_Key, e EmptyDefaultInfoEnvironment[TReference, TMetadata]) (PatchedEmptyDefaultInfoValue, error) {
	allBuiltinsModulesNames := e.GetBuiltinsModuleNamesValue(&model_analysis_pb.BuiltinsModuleNames_Key{})
	defaultInfoProviderIdentifierStr := defaultInfoProviderIdentifier.String()
	defaultInfoProviderValue := e.GetCompiledBzlFileGlobalValue(&model_analysis_pb.CompiledBzlFileGlobal_Key{
		Identifier: defaultInfoProviderIdentifierStr,
	})
	if !allBuiltinsModulesNames.IsSet() || !defaultInfoProviderValue.IsSet() {
		return PatchedEmptyDefaultInfoValue{}, evaluation.ErrMissingDependency
	}

	// Construct the provider object.
	defaultInfoProviderValueKind, ok := defaultInfoProviderValue.Message.Global.GetKind().(*model_starlark_pb.Value_Provider)
	if !ok {
		return PatchedEmptyDefaultInfoValue{}, fmt.Errorf("%#v is not a provider", defaultInfoProviderIdentifierStr)
	}
	defaultInfoProvider, err := model_starlark.DecodeProvider[TReference, TMetadata](model_core.Nested(defaultInfoProviderValue, defaultInfoProviderValueKind.Provider))
	if err != nil {
		return PatchedEmptyDefaultInfoValue{}, fmt.Errorf("failed to decode provider %#v: %w", defaultInfoProviderIdentifierStr, err)
	}

	// Call into the DefaultInfo provider to create a new instance.
	thread := c.newStarlarkThread(ctx, e, allBuiltinsModulesNames.Message.BuiltinsModuleNames)
	identifierGenerator, err := c.getReferenceEqualIdentifierGenerator(model_core.NewSimpleMessage[TReference](proto.Message(key)))
	if err != nil {
		return PatchedEmptyDefaultInfoValue{}, fmt.Errorf("failed to obtain identifier generator for reference equal values: %w", err)
	}
	thread.SetLocal(model_starlark.ReferenceEqualIdentifierGeneratorKey, identifierGenerator)

	defaultInfo, err := defaultInfoProvider.Instantiate(thread, nil, nil)
	if err != nil {
		var evalErr *starlark.EvalError
		if errors.As(err, &evalErr) {
			return PatchedEmptyDefaultInfoValue{}, errors.New(evalErr.Backtrace())
		}
		return PatchedEmptyDefaultInfoValue{}, err
	}

	// Encode the DefaultInfo provider instance.
	encodedDefaultInfo, _, err := defaultInfo.Encode(map[starlark.Value]struct{}{}, c.getValueEncodingOptions(e, nil))
	if err != nil {
		return PatchedEmptyDefaultInfoValue{}, fmt.Errorf("failed to encode DefaultInfo provider instance: %w", err)
	}
	return model_core.NewPatchedMessage(
		&model_analysis_pb.EmptyDefaultInfo_Value{
			DefaultInfo: encodedDefaultInfo.Message,
		},
		model_core.MapReferenceMetadataToWalkers(encodedDefaultInfo.Patcher),
	), nil
}
