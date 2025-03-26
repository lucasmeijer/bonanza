package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

var configSettingInfoProviderIdentifier = label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%ConfigSettingInfo")

func (c *baseComputer[TReference, TMetadata]) ComputeSelectValue(ctx context.Context, key model_core.Message[*model_analysis_pb.Select_Key, TReference], e SelectEnvironment[TReference, TMetadata]) (PatchedSelectValue, error) {
	missingDependencies := false
	for _, conditionIdentifier := range key.Message.ConditionIdentifiers {
		visibleTargetValue := e.GetVisibleTargetValue(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_analysis_pb.VisibleTarget_Key{
					FromPackage: key.Message.FromPackage,
					ToLabel:     conditionIdentifier,
				},
			),
		)
		if !visibleTargetValue.IsSet() {
			missingDependencies = true
			continue
		}
		targetLabel := visibleTargetValue.Message.Label

		_, err := getProviderFromConfiguredTarget(
			e,
			targetLabel,
			model_core.NewSimplePatchedMessage[model_core.WalkableReferenceMetadata, *model_core_pb.Reference](nil),
			configSettingInfoProviderIdentifier,
		)
		if err != nil {
			if errors.Is(err, evaluation.ErrMissingDependency) {
				missingDependencies = true
				continue
			}
			return PatchedSelectValue{}, fmt.Errorf("failed to obtain ConfigSettingInfo provider for target %#v: %w", targetLabel, err)
		}
	}
	if missingDependencies {
		return PatchedSelectValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.Select_Value{}), nil
}
