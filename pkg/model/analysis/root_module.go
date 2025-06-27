package analysis

import (
	"context"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	"bonanza.build/pkg/storage/dag"
)

func (c *baseComputer[TReference, TMetadata]) ComputeRootModuleValue(ctx context.Context, key *model_analysis_pb.RootModule_Key, e RootModuleEnvironment[TReference, TMetadata]) (PatchedRootModuleValue, error) {
	buildSpecification := e.GetBuildSpecificationValue(&model_analysis_pb.BuildSpecification_Key{})
	if !buildSpecification.IsSet() {
		return PatchedRootModuleValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.RootModule_Value{
		RootModuleName:                  buildSpecification.Message.BuildSpecification.RootModuleName,
		IgnoreRootModuleDevDependencies: buildSpecification.Message.BuildSpecification.IgnoreRootModuleDevDependencies,
	}), nil
}
