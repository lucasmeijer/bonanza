package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

var configSettingInfoProviderIdentifier = label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%ConfigSettingInfo")

func (c *baseComputer[TReference, TMetadata]) ComputeSelectValue(ctx context.Context, key model_core.Message[*model_analysis_pb.Select_Key, TReference], e SelectEnvironment[TReference, TMetadata]) (PatchedSelectValue, error) {
	missingDependencies := false
	var platformConstraints []*model_analysis_pb.Constraint
CheckConditions:
	for _, conditionIdentifier := range key.Message.ConditionIdentifiers {
		configSettingInfo, err := getProviderFromVisibleConfiguredTarget(
			e,
			key.Message.FromPackage,
			conditionIdentifier,
			model_core.NewSimpleMessage[TReference]((*model_core_pb.Reference)(nil)),
			e,
			configSettingInfoProviderIdentifier,
		)
		if err != nil {
			if errors.Is(err, evaluation.ErrMissingDependency) {
				missingDependencies = true
				continue
			}
			return PatchedSelectValue{}, fmt.Errorf("failed to obtain ConfigSettingInfo provider of target %#v: %w", conditionIdentifier, err)
		}

		configSettingConstraintsField, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, configSettingInfo, "constraints")
		if err != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to obtain constraints field of ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, err)
		}
		configSettingConstraints, err := c.extractFromPlatformInfoConstraints(ctx, configSettingConstraintsField)
		if err != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to extract constraints from ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, err)
		}
		if len(configSettingConstraints) > 0 {
			if platformConstraints == nil {
				platformInfo, err := getTargetPlatformInfoProvider(
					e,
					model_core.NewSimpleMessage[TReference]((*model_core_pb.Reference)(nil)),
				)
				if err != nil {
					if errors.Is(err, evaluation.ErrMissingDependency) {
						missingDependencies = true
						continue
					}
					return PatchedSelectValue{}, fmt.Errorf("failed to obtain PlatformInfo provider of target platform: %w", err)
				}

				platformConstraintsField, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, platformInfo, "constraints")
				if err != nil {
					return PatchedSelectValue{}, fmt.Errorf("failed to obtain constraints field of PlatformInfo provider of target platform: %w", err)
				}
				platformConstraints, err = c.extractFromPlatformInfoConstraints(ctx, platformConstraintsField)
				if err != nil {
					return PatchedSelectValue{}, fmt.Errorf("failed to extract constraints from ConfigSettingInfo provider of target platform: %w", err)
				}
			}
			if !constraintsAreCompatible(platformConstraints, configSettingConstraints) {
				// Condition contains constraints that
				// are incompatible with the target
				// platform. Skip this condition.
				continue CheckConditions
			}
		}
	}
	if missingDependencies {
		return PatchedSelectValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.Select_Value{}), nil
}
