package analysis

import (
	"context"
	"errors"

	"github.com/buildbarn/bonanza/pkg/evaluation"
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

		if _, err := getStarlarkFileProperties(ctx, e, model_core.Nested(element, elementFile.File)); err != nil {
			if errors.Is(err, evaluation.ErrMissingDependency) {
				missingDependencies = true
				continue
			}
			return PatchedTargetCompletionValue{}, err
		}
	}
	if missingDependencies {
		return PatchedTargetCompletionValue{}, evaluation.ErrMissingDependency
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.TargetCompletion_Value{}), nil
}
