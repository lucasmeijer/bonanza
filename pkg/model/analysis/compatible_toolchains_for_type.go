package analysis

import (
	"context"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

func (c *baseComputer[TReference, TMetadata]) getConfigurationByReference(ctx context.Context, configurationReference model_core.Message[*model_core_pb.Reference, TReference]) (model_core.Message[*model_analysis_pb.Configuration, TReference], error) {
	if configurationReference.Message == nil {
		// Empty configuration.
		return model_core.NewSimpleMessage[TReference](&model_analysis_pb.Configuration{}), nil
	}
	return model_parser.Dereference(ctx, c.configurationReader, configurationReference)
}

var commandLineOptionPlatformsLabel = label.MustNewCanonicalLabel("@@bazel_tools+//command_line_option:platforms")

func (c *baseComputer[TReference, TMetadata]) ComputeCompatibleToolchainsForTypeValue(ctx context.Context, key model_core.Message[*model_analysis_pb.CompatibleToolchainsForType_Key, TReference], e CompatibleToolchainsForTypeEnvironment[TReference, TMetadata]) (PatchedCompatibleToolchainsForTypeValue, error) {
	patchedConfigurationReference := model_core.NewPatchedMessageFromExistingCaptured(
		e,
		model_core.NewNestedMessage(key, key.Message.ConfigurationReference),
	)
	platformLabel := e.GetVisibleTargetValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.VisibleTarget_Key{
				ConfigurationReference: patchedConfigurationReference.Message,
				FromPackage:            commandLineOptionPlatformsLabel.GetCanonicalPackage().String(),
				ToLabel:                commandLineOptionPlatformsLabel.String(),
			},
			model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
		),
	)
	registeredToolchains := e.GetRegisteredToolchainsForTypeValue(&model_analysis_pb.RegisteredToolchainsForType_Key{
		ToolchainType: key.Message.ToolchainType,
	})
	if !platformLabel.IsSet() || !registeredToolchains.IsSet() {
		return PatchedCompatibleToolchainsForTypeValue{}, evaluation.ErrMissingDependency
	}

	platformInfoProvider, err := getProviderFromConfiguredTarget(
		e,
		platformLabel.Message.Label,
		model_core.NewSimplePatchedMessage[model_core.WalkableReferenceMetadata, *model_core_pb.Reference](nil),
		platformInfoProviderIdentifier,
	)
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("failed to obtain PlatformInfo provider for target %#v: %w", platformLabel, err)
	}
	constraintsValue, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, platformInfoProvider, "constraints")
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("failed to obtain constraints field of PlatformInfo provider for target %#v: %w", err)
	}
	constraints, err := c.extractFromPlatformInfoConstraints(ctx, constraintsValue)
	if err != nil {
		return PatchedCompatibleToolchainsForTypeValue{}, fmt.Errorf("failed to extract constraints from PlatformInfo provider for target %#v", platformLabel)
	}

	allToolchains := registeredToolchains.Message.Toolchains
	var compatibleToolchains []*model_analysis_pb.RegisteredToolchain
	for _, toolchain := range allToolchains {
		if constraintsAreCompatible(constraints, toolchain.TargetCompatibleWith) {
			compatibleToolchains = append(compatibleToolchains, toolchain)
		}
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.CompatibleToolchainsForType_Value{
		Toolchains: compatibleToolchains,
	}), nil
}
