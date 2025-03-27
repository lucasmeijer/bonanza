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
	getProviderFromVisibleConfiguredTargetEnvironment[TReference]
}

func getTargetPlatformInfoProvider[TReference object.BasicReference, TMetadata BaseComputerReferenceMetadata](
	e getTargetPlatformInfoProviderEnvironment[TReference, TMetadata],
	configurationReference model_core.Message[*model_core_pb.Reference, TReference],
) (model_core.Message[*model_starlark_pb.Struct_Fields, TReference], error) {
	platformsLabelStr := commandLineOptionPlatformsLabel.String()
	platformInfoProvider, err := getProviderFromVisibleConfiguredTarget(
		e,
		commandLineOptionPlatformsLabel.GetCanonicalPackage().String(),
		platformsLabelStr,
		model_core.NewSimpleMessage[TReference]((*model_core_pb.Reference)(nil)),
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

	platformInfoProvider, err := getTargetPlatformInfoProvider(e, model_core.Nested(key, key.Message.ConfigurationReference))
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
