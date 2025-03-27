package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

var configSettingInfoProviderIdentifier = label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%ConfigSettingInfo")

func (c *baseComputer[TReference, TMetadata]) ComputeSelectValue(ctx context.Context, key model_core.Message[*model_analysis_pb.Select_Key, TReference], e SelectEnvironment[TReference, TMetadata]) (PatchedSelectValue, error) {
	configurationReference := model_core.Nested(key, key.Message.ConfigurationReference)
	configuration, err := model_parser.MaybeDereference(ctx, c.configurationReader, configurationReference)
	if err != nil {
		return PatchedSelectValue{}, err
	}

	missingDependencies := false
	var platformConstraints []*model_analysis_pb.Constraint
CheckConditions:
	for _, conditionIdentifier := range key.Message.ConditionIdentifiers {
		configSettingInfo, configSettingLabelStr, err := getProviderFromVisibleConfiguredTarget(
			e,
			key.Message.FromPackage,
			conditionIdentifier,
			configurationReference,
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

		// Check target platform constraints.
		configSettingConstraintsField, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, configSettingInfo, "constraints")
		if err != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to obtain \"constraints\" field of ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, err)
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

		// Check flag values.
		flagValuesField, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, configSettingInfo, "flag_values")
		if err != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to obtain \"flag_values\" field of ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, err)
		}
		flagValuesDict, ok := flagValuesField.Message.GetKind().(*model_starlark_pb.Value_Dict)
		if !ok {
			return PatchedSelectValue{}, fmt.Errorf("\"flag_values\" field of ConfigSettingInfo provider of config setting %#v is not a dict", conditionIdentifier)
		}
		configSettingLabel, err := label.NewCanonicalLabel(configSettingLabelStr)
		if err != nil {
			return PatchedSelectValue{}, fmt.Errorf("invalid condition identifier %#v: %w", configSettingLabel)
		}
		var errIter error
		for key := range model_starlark.AllDictLeafEntries(
			ctx,
			c.valueReaders.Dict,
			model_core.Nested(flagValuesField, flagValuesDict.Dict),
			&errIter,
		) {
			buildSettingLabel, ok := key.Message.GetKind().(*model_starlark_pb.Value_Label)
			if !ok {
				return PatchedSelectValue{}, fmt.Errorf("\"flag_values\" field of ConfigSettingInfo provider of config setting %#v contains a key that is not a label", conditionIdentifier)
			}
			_, err := c.getBuildSettingValue(
				ctx,
				e,
				configSettingLabel.GetCanonicalPackage(),
				buildSettingLabel.Label,
				configuration,
			)
			if err != nil {
				if errors.Is(err, evaluation.ErrMissingDependency) {
					missingDependencies = true
					continue
				}
				return PatchedSelectValue{}, err
			}

			// TODO: Compare!
		}
		if errIter != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to iterate \"flag_values\" field of ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, errIter)
		}
	}
	if missingDependencies {
		return PatchedSelectValue{}, evaluation.ErrMissingDependency
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.Select_Value{}), nil
}
