package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
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

		configuredTarget := e.GetConfiguredTargetValue(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_analysis_pb.ConfiguredTarget_Key{
					Label: targetLabel,
				},
			),
		)
		if !configuredTarget.IsSet() {
			missingDependencies = true
			continue
		}

		configSettingInfoProviderIdentifierStr := configSettingInfoProviderIdentifier.String()
		constraintValueInfoProviderIdentifierStr := constraintValueInfoProviderIdentifier.String()
		providerInstances := configuredTarget.Message.ProviderInstances
		if _, ok := sort.Find(
			len(providerInstances),
			func(i int) int {
				return strings.Compare(configSettingInfoProviderIdentifierStr, providerInstances[i].ProviderInstanceProperties.GetProviderIdentifier())
			},
		); ok {
			// TODO!
		} else if _, ok := sort.Find(
			len(providerInstances),
			func(i int) int {
				return strings.Compare(constraintValueInfoProviderIdentifierStr, providerInstances[i].ProviderInstanceProperties.GetProviderIdentifier())
			},
		); ok {
			// TODO!
		} else {
			return PatchedSelectValue{}, fmt.Errorf("target %#v does not provide ConfigSettingInfo or ConstraintValueInfo", targetLabel)
		}

	}
	if missingDependencies {
		return PatchedSelectValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.Select_Value{}), nil
}
