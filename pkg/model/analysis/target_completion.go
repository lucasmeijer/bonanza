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
	"github.com/buildbarn/bonanza/pkg/storage/object"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetCompletionValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetCompletion_Key, TReference], e TargetCompletionEnvironment[TReference, TMetadata]) (PatchedTargetCompletionValue, error) {
	// TODO: This should also respect --output_groups.
	defaultInfo, err := getProviderFromConfiguredTarget(
		e,
		key.Message.Label,
		model_core.Patch(e, model_core.Nested(key, key.Message.ConfigurationReference)),
		defaultInfoProviderIdentifier,
	)
	if err != nil {
		return PatchedTargetCompletionValue{}, err
	}

	files, err := model_starlark.GetStructFieldValue(ctx, c.valueReaders.List, defaultInfo, "files")
	if err != nil {
		return PatchedTargetCompletionValue{}, err
	}
	filesDepset, ok := files.Message.Kind.(*model_starlark_pb.Value_Depset)
	if !ok {
		return PatchedTargetCompletionValue{}, errors.New("\"files\" field of DefaultInfo provider is not a depset")
	}

	var errIter error
	missingDependencies := false
	for element := range model_starlark.AllListLeafElementsSkippingDuplicateParents(
		ctx,
		c.valueReaders.List,
		model_core.Nested(files, filesDepset.Depset.Elements),
		map[object.LocalReference]struct{}{},
		&errIter,
	) {
		elementFile, ok := element.Message.Kind.(*model_starlark_pb.Value_File)
		if !ok {
			return PatchedTargetCompletionValue{}, errors.New("\"files\" field of DefaultInfo provider contains an element that is not a File")
		}

		file := elementFile.File
		canonicalPackage, err := label.NewCanonicalPackage(file.Package)
		if err != nil {
			return PatchedTargetCompletionValue{}, fmt.Errorf("invalid package %#v: %w", file.Package, err)
		}

		if owner := file.Owner; owner == nil {
			// File is a source file. Ensure it exists.
			packageRelativePath, err := label.NewTargetName(file.PackageRelativePath)
			if err != nil {
				return PatchedTargetCompletionValue{}, fmt.Errorf("invalid package relative path %#v: %w", file.PackageRelativePath, err)
			}
			fileProperties := e.GetFilePropertiesValue(
				&model_analysis_pb.FileProperties_Key{
					CanonicalRepo: canonicalPackage.GetCanonicalRepo().String(),
					Path:          canonicalPackage.AppendTargetName(packageRelativePath).GetRepoRelativePath(),
				},
			)
			if !fileProperties.IsSet() {
				missingDependencies = true
				continue
			}
		} else {
			// File is an output file. Ensure it gets built.
			targetName, err := label.NewTargetName(owner.TargetName)
			if err != nil {
				return PatchedTargetCompletionValue{}, fmt.Errorf("invalid target name %#v: %w", owner.TargetName, err)
			}

			configurationReference := model_core.Patch(e, model_core.Nested(element, owner.ConfigurationReference))
			targetOutput := e.GetTargetOutputValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.TargetOutput_Key{
						TargetLabel:            canonicalPackage.AppendTargetName(targetName).String(),
						PackageRelativePath:    file.PackageRelativePath,
						ConfigurationReference: configurationReference.Message,
					},
					model_core.MapReferenceMetadataToWalkers(configurationReference.Patcher),
				),
			)
			if !targetOutput.IsSet() {
				missingDependencies = true
				continue
			}
		}
	}
	if missingDependencies {
		return PatchedTargetCompletionValue{}, evaluation.ErrMissingDependency
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.TargetCompletion_Value{}), nil
}
