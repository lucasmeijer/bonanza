package analysis

import (
	"context"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	"bonanza.build/pkg/storage/dag"
)

func (c *baseComputer[TReference, TMetadata]) ComputeModuleRegistryUrlsValue(ctx context.Context, key *model_analysis_pb.ModuleRegistryUrls_Key, e ModuleRegistryUrlsEnvironment[TReference, TMetadata]) (PatchedModuleRegistryUrlsValue, error) {
	buildSpecification := e.GetBuildSpecificationValue(&model_analysis_pb.BuildSpecification_Key{})
	if !buildSpecification.IsSet() {
		return PatchedModuleRegistryUrlsValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.ModuleRegistryUrls_Value{
		RegistryUrls: buildSpecification.Message.BuildSpecification.ModuleRegistryUrls,
	}), nil
}
