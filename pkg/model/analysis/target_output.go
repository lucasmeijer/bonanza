package analysis

import (
	"context"
	"errors"
	"strings"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetOutputValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetOutput_Key, TReference], e TargetOutputEnvironment[TReference, TMetadata]) (PatchedTargetOutputValue, error) {
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
	if !configuredTarget.IsSet() {
		return PatchedTargetOutputValue{}, evaluation.ErrMissingDependency
	}

	packageRelativePath := key.Message.PackageRelativePath
	output, err := btree.Find(
		ctx,
		c.configuredTargetOutputReader,
		model_core.Nested(configuredTarget, configuredTarget.Message.Outputs),
		func(entry model_core.Message[*model_analysis_pb.ConfiguredTarget_Value_Output, TReference]) (int, *model_core_pb.DecodableReference) {
			switch level := entry.Message.Level.(type) {
			case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_:
				return strings.Compare(packageRelativePath, level.Leaf.PackageRelativePath), nil
			case *model_analysis_pb.ConfiguredTarget_Value_Output_Parent_:
				return strings.Compare(packageRelativePath, level.Parent.FirstPackageRelativePath), level.Parent.Reference
			default:
				return 0, nil
			}
		},
	)
	if err != nil {
		return PatchedTargetOutputValue{}, err
	}
	if !output.IsSet() {
		return PatchedTargetOutputValue{}, errors.New("target does not yield an output with the provided package relative path")
	}
	outputLevel, ok := output.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_)
	if !ok {
		return PatchedTargetOutputValue{}, errors.New("output is not a leaf")
	}

	patchedDefinition := model_core.Patch(e, model_core.Nested(output, outputLevel.Leaf.Definition))
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetOutput_Value{
			Definition: patchedDefinition.Message,
		},
		model_core.MapReferenceMetadataToWalkers(patchedDefinition.Patcher),
	), nil
}
