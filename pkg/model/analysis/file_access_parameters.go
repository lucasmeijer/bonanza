package analysis

import (
	"context"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	"bonanza.build/pkg/storage/dag"
)

func (c *baseComputer[TReference, TMetadata]) ComputeFileAccessParametersValue(ctx context.Context, key *model_analysis_pb.FileAccessParameters_Key, e FileAccessParametersEnvironment[TReference, TMetadata]) (PatchedFileAccessParametersValue, error) {
	fileCreationParameters := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !fileCreationParameters.IsSet() {
		return PatchedFileAccessParametersValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.FileAccessParameters_Value{
		FileAccessParameters: fileCreationParameters.Message.FileCreationParameters.GetAccess(),
	}), nil
}
