package analysis

import (
	"context"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"
)

var commandLineOptionPlatformsLabel = label.MustNewCanonicalLabel("@@bazel_tools+//command_line_option:platforms")

type getTargetPlatformInfoProviderEnvironment[TReference any, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]
	getProviderFromVisibleConfiguredTargetEnvironment[TReference]
}

func getTargetPlatformInfoProvider[TReference object.BasicReference, TMetadata BaseComputerReferenceMetadata](
	e getTargetPlatformInfoProviderEnvironment[TReference, TMetadata],
	configurationReference model_core.Message[*model_core_pb.Reference, TReference],
) (model_core.Message[*model_starlark_pb.Struct_Fields, TReference], error) {
	platformsLabelStr := commandLineOptionPlatformsLabel.String()
	platformInfoProvider, _, err := getProviderFromVisibleConfiguredTarget(
		e,
		commandLineOptionPlatformsLabel.GetCanonicalPackage().String(),
		platformsLabelStr,
		configurationReference,
		e,
		platformInfoProviderIdentifier,
	)
	if err != nil {
		return model_core.Message[*model_starlark_pb.Struct_Fields, TReference]{}, fmt.Errorf("failed to obtain PlatformInfo provider for target %#v: %w", platformsLabelStr, err)
	}
	return platformInfoProvider, nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeCompatibleToolchainsForTypeValue(ctx context.Context, key model_core.Message[*model_analysis_pb.CompatibleToolchainsForType_Key, TReference], e CompatibleToolchainsForTypeEnvironment[TReference, TMetadata]) (PatchedCompatibleToolchainsForTypeValue, error) {
	registeredToolchains := e.GetRegisteredToolchainsForTypeValue(&model_analysis_pb.RegisteredToolchainsForType_Key{
		ToolchainType: key.Message.ToolchainType,
	})
	if !registeredToolchains.IsSet() {
		return PatchedCompatibleToolchainsForTypeValue{}, evaluation.ErrMissingDependency
	}

	configurationReference := model_core.Nested(key, key.Message.ConfigurationReference)
	platformInfoProvider, err := getTargetPlatformInfoProvider(e, configurationReference)
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, err
	}

	constraintsValue, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, platformInfoProvider, "constraints")
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("failed to obtain constraints field of PlatformInfo provider of target platform: %w", err)
	}
	constraints, err := c.extractFromPlatformInfoConstraints(ctx, constraintsValue)
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("failed to extract constraints from PlatformInfo provider of target platform: %w", err)
	}

	missingDependencies := false
	allToolchains := registeredToolchains.Message.Toolchains
	var compatibleToolchains []*model_analysis_pb.RegisteredToolchain
FindCompatibleToolchains:
	for _, toolchain := range allToolchains {
		if !constraintsAreCompatible(constraints, toolchain.TargetCompatibleWith) {
			// Missing incompatible constraint value.
			continue FindCompatibleToolchains
		}

		for _, targetSetting := range toolchain.TargetSettings {
			targetSettingLabel, err := label.NewCanonicalLabel(targetSetting)
			if err != nil {
				return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("invalid target setting label %#v: %w", targetSettingLabel, err)
			}
			patchedConfigurationReference := model_core.Patch(e, configurationReference)
			selectValue := e.GetSelectValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.Select_Key{
						ConditionIdentifiers:   []string{targetSetting},
						ConfigurationReference: patchedConfigurationReference.Message,
						// Visibility for target settings is already
						// validated when configuring the toolchain target.
						FromPackage: targetSettingLabel.GetCanonicalPackage().String(),
					},
					model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
				),
			)
			if !selectValue.IsSet() {
				missingDependencies = true
				continue
			}
			if len(selectValue.Message.ConditionIndices) == 0 {
				// Incompatible target setting.
				continue FindCompatibleToolchains
			}
		}
		compatibleToolchains = append(compatibleToolchains, toolchain)
	}
	if missingDependencies {
		return PatchedCompatibleToolchainsForTypeValue{}, evaluation.ErrMissingDependency
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.CompatibleToolchainsForType_Value{
		Toolchains: compatibleToolchains,
	}), nil
}
