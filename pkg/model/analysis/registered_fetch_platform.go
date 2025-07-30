package analysis

import (
	"context"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	"bonanza.build/pkg/storage/dag"
)

func (c *baseComputer[TReference, TMetadata]) ComputeRegisteredFetchPlatformValue(ctx context.Context, key *model_analysis_pb.RegisteredFetchPlatform_Key, e RegisteredFetchPlatformEnvironment[TReference, TMetadata]) (PatchedRegisteredFetchPlatformValue, error) {
	buildSpecification := e.GetBuildSpecificationValue(&model_analysis_pb.BuildSpecification_Key{})
	if !buildSpecification.IsSet() {
		return PatchedRegisteredFetchPlatformValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.RegisteredFetchPlatform_Value{
		FetchPlatformPkixPublicKey: buildSpecification.Message.BuildSpecification.GetFetchPlatformPkixPublicKey(),
	}), nil
}
