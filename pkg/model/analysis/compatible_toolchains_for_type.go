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
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"
)

func (c *baseComputer[TReference, TMetadata]) getConfigurationByReference(ctx context.Context, configurationReference model_core.Message[*model_core_pb.Reference, TReference]) (model_core.Message[*model_analysis_pb.Configuration, TReference], error) {
	if configurationReference.Message == nil {
		// Empty configuration.
		return model_core.NewSimpleMessage[TReference](&model_analysis_pb.Configuration{}), nil
	}
	return model_parser.Dereference(ctx, c.configurationReader, configurationReference)
}

var commandLineOptionPlatformsLabel = label.MustNewCanonicalLabel("@@bazel_tools+//command_line_option:platforms")

type getTargetPlatformInfoProviderEnvironment[TReference any, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]
	getProviderFromConfiguredTargetEnvironment[TReference]

	GetVisibleTargetValue(model_core.PatchedMessage[*model_analysis_pb.VisibleTarget_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.VisibleTarget_Value, TReference]
}

func getTargetPlatformInfoProvider[TReference object.BasicReference, TMetadata BaseComputerReferenceMetadata](
	e getTargetPlatformInfoProviderEnvironment[TReference, TMetadata],
	configurationReference model_core.Message[*model_core_pb.Reference, TReference],
) (model_core.Message[*model_starlark_pb.Struct_Fields, TReference], error) {
	patchedConfigurationReference := model_core.NewPatchedMessageFromExistingCaptured(e, configurationReference)
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
	if !platformLabel.IsSet() {
		return model_core.Message[*model_starlark_pb.Struct_Fields, TReference]{}, evaluation.ErrMissingDependency
	}

	platformInfoProvider, err := getProviderFromConfiguredTarget(
		e,
		platformLabel.Message.Label,
		model_core.NewSimplePatchedMessage[model_core.WalkableReferenceMetadata, *model_core_pb.Reference](nil),
		platformInfoProviderIdentifier,
	)
	if err != nil {
		return model_core.Message[*model_starlark_pb.Struct_Fields, TReference]{}, fmt.Errorf("failed to obtain PlatformInfo provider for target %#v: %w", platformLabel, err)
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

	platformInfoProvider, err := getTargetPlatformInfoProvider(e, model_core.NewNestedMessage(key, key.Message.ConfigurationReference))
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
