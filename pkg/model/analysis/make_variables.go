package analysis

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

var templateVariableInfoProviderIdentifier = label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%TemplateVariableInfo")

func (c *baseComputer[TReference, TMetadata]) ComputeMakeVariablesValue(ctx context.Context, key model_core.Message[*model_analysis_pb.MakeVariables_Key, TReference], e MakeVariablesEnvironment[TReference, TMetadata]) (PatchedMakeVariablesValue, error) {
	allVariables := map[string]string{}
	missingDependencies := false
	configurationReference := model_core.Nested(key, key.Message.ConfigurationReference)
	for _, toolchainLabel := range append([]string{"@@bazel_tools+//tools/make:default_make_variables"}, key.Message.Toolchains...) {
		// Obtain TemplateVariableInfo of the provided toolchain.
		templateVariableInfoProvider, _, err := getProviderFromVisibleConfiguredTarget(
			e,
			key.Message.FromPackage,
			toolchainLabel,
			configurationReference,
			e,
			templateVariableInfoProviderIdentifier,
		)
		if err != nil {
			if errors.Is(err, evaluation.ErrMissingDependency) {
				missingDependencies = true
				continue
			}
			return PatchedMakeVariablesValue{}, fmt.Errorf("failed to obtain TemplateVariableInfo provider of toolchain %#v: %w", toolchainLabel, err)
		}

		// Add all variables contained in the TemplateVariableInfo
		// to the results.
		variables, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, templateVariableInfoProvider, "variables")
		if err != nil {
			return PatchedMakeVariablesValue{}, fmt.Errorf("failed to obtain \"variables\" field of TemplateVariableInfo provider of toolchain %#v: %w", toolchainLabel, err)
		}
		variablesDict, ok := variables.Message.GetKind().(*model_starlark_pb.Value_Dict)
		if !ok {
			return PatchedMakeVariablesValue{}, fmt.Errorf("\"variables\" field of TemplateVariableInfo provider of toolchain %#v is not a dict", toolchainLabel)
		}

		var errIter error
		for key, value := range model_starlark.AllDictLeafEntries(
			ctx,
			c.valueReaders.Dict,
			model_core.Nested(variables, variablesDict.Dict),
			&errIter,
		) {
			keyStr, ok := key.Message.GetKind().(*model_starlark_pb.Value_Str)
			if !ok {
				return PatchedMakeVariablesValue{}, fmt.Errorf("key of Make variable provided of toolchain %#v is not a string", toolchainLabel)
			}
			valueStr, ok := value.Message.GetKind().(*model_starlark_pb.Value_Str)
			if !ok {
				return PatchedMakeVariablesValue{}, fmt.Errorf("value of Make variable %#v provided by toolchain %#v is not a string", keyStr.Str, toolchainLabel)
			}
			if _, ok := allVariables[keyStr.Str]; ok {
				return PatchedMakeVariablesValue{}, fmt.Errorf("Make variable %#v is provided by multiple toolchains, including %#v", keyStr.Str, toolchainLabel)
			}
			allVariables[keyStr.Str] = valueStr.Str
		}
		if errIter != nil {
			return PatchedMakeVariablesValue{}, fmt.Errorf("failed to iterate \"variables\" field of TemplateVariableInfo provide of toolchain %#v: %w", toolchainLabel, errIter)
		}
	}
	if missingDependencies {
		return PatchedMakeVariablesValue{}, evaluation.ErrMissingDependency
	}

	// Return all variables in sorted order.
	result := make([]*model_analysis_pb.MakeVariables_Value_MakeVariable, 0, len(allVariables))
	for _, key := range slices.Sorted(maps.Keys(allVariables)) {
		result = append(result, &model_analysis_pb.MakeVariables_Value_MakeVariable{
			Key:   key,
			Value: allVariables[key],
		})
	}
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
		&model_analysis_pb.MakeVariables_Value{
			Variables: result,
		},
	), nil
}
