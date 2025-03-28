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
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

var configSettingInfoProviderIdentifier = label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%ConfigSettingInfo")

func (c *baseComputer[TReference, TMetadata]) ComputeSelectValue(ctx context.Context, key model_core.Message[*model_analysis_pb.Select_Key, TReference], e SelectEnvironment[TReference, TMetadata]) (PatchedSelectValue, error) {
	fromPackage, err := label.NewCanonicalPackage(key.Message.FromPackage)
	if err != nil {
		return PatchedSelectValue{}, fmt.Errorf("invalid package: %w", err)
	}
	configurationReference := model_core.Nested(key, key.Message.ConfigurationReference)
	missingDependencies := false
	var platformConstraints []*model_analysis_pb.Constraint
	var matchingIndices []uint32
CheckConditions:
	for i, conditionIdentifier := range key.Message.ConditionIdentifiers {
		configSettingInfo, configSettingLabelStr, err := getProviderFromVisibleConfiguredTarget(
			e,
			fromPackage.String(),
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
				platformInfo, err := getTargetPlatformInfoProvider(e, configurationReference)
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
		for key, value := range model_starlark.AllDictLeafEntries(
			ctx,
			c.valueReaders.Dict,
			model_core.Nested(flagValuesField, flagValuesDict.Dict),
			&errIter,
		) {
			buildSettingLabel, ok := key.Message.GetKind().(*model_starlark_pb.Value_Label)
			if !ok {
				return PatchedSelectValue{}, fmt.Errorf("\"flag_values\" field of ConfigSettingInfo provider of config setting %#v contains a key that is not a label", conditionIdentifier)
			}
			expectedValue, ok := value.Message.GetKind().(*model_starlark_pb.Value_Str)
			if !ok {
				return PatchedSelectValue{}, fmt.Errorf("key %#v of \"flag_values\" field of ConfigSettingInfo provider of config setting %#v is not a string", buildSettingLabel.Label, conditionIdentifier)
			}

			actualValue, err := c.getBuildSettingValue(
				ctx,
				e,
				configSettingLabel.GetCanonicalPackage(),
				buildSettingLabel.Label,
				configurationReference,
			)
			if err != nil {
				if errors.Is(err, evaluation.ErrMissingDependency) {
					missingDependencies = true
					continue
				}
				return PatchedSelectValue{}, err
			}

			equal, err := c.compareBuildSettingValue(e, expectedValue.Str, actualValue, fromPackage)
			if err != nil {
				return PatchedSelectValue{}, fmt.Errorf("failed to compare key %#v of \"flag_values\" field of ConfigSettingInfo provider of config setting %#v: %w", buildSettingLabel.Label, conditionIdentifier, err)
			}
			if !equal {
				continue CheckConditions
			}
		}
		if errIter != nil {
			return PatchedSelectValue{}, fmt.Errorf("failed to iterate \"flag_values\" field of ConfigSettingInfo provider of config setting %#v: %w", conditionIdentifier, errIter)
		}

		// TODO: Check specializations!
		matchingIndices = append(matchingIndices, uint32(i))
	}
	if missingDependencies {
		return PatchedSelectValue{}, evaluation.ErrMissingDependency
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
		&model_analysis_pb.Select_Value{
			ConditionIndices: matchingIndices,
		},
	), nil
}

func (c *baseComputer[TReference, TMetadata]) compareBuildSettingValue(e resolveApparentEnvironment[TReference], expectedValue string, actualValue model_core.Message[*model_starlark_pb.Value, TReference], fromPackage label.CanonicalPackage) (bool, error) {
	switch typedValue := actualValue.Message.GetKind().(type) {
	case *model_starlark_pb.Value_Bool:
		switch expectedValue {
		case "0", "false", "False":
			return !typedValue.Bool, nil
		case "1", "true", "True":
			return typedValue.Bool, nil
		default:
			return false, fmt.Errorf("boolean values can only be compared against \"0\", \"1\", \"false\", \"true\", \"False\" and \"True\", not %#v", expectedValue)
		}
		return false, nil
	case *model_starlark_pb.Value_Label:
		// Parse label to obtain a canonical representation.
		apparentLabel, err := fromPackage.AppendLabel(expectedValue)
		if err != nil {
			return false, fmt.Errorf("invalid label %#v: %w", expectedValue, err)
		}
		// TODO: Apparently Bazel allows this to succeed for
		// unknown repo names. We should likely support that as
		// well?
		canonicalLabel, err := resolveApparent(e, fromPackage.GetCanonicalRepo(), apparentLabel)
		if err != nil {
			return false, fmt.Errorf("failed to resolve label %#v: %w", expectedValue, err)
		}
		return canonicalLabel.String() == typedValue.Label, nil
	case *model_starlark_pb.Value_Str:
		return expectedValue == typedValue.Str, nil
	default:
		return false, errors.New("build setting value is of an unknown type")
	}
}
