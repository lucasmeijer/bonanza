package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionInputRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionInputRoot_Key, TReference], e TargetActionInputRootEnvironment[TReference, TMetadata]) (PatchedTargetActionInputRootValue, error) {
	id := model_core.Nested(key, key.Message.Id)
	if id.Message == nil {
		return PatchedTargetActionInputRootValue{}, errors.New("no target action identifier specified")
	}
	targetLabel, err := label.NewCanonicalLabel(id.Message.Label)
	if err != nil {
		return PatchedTargetActionInputRootValue{}, errors.New("invalid target label")
	}

	patchedID := model_core.Patch(e, id)
	action := e.GetTargetActionValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetAction_Key{
				Id: patchedID.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID.Patcher),
		),
	)
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	fileCreationParametersMessage := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !action.IsSet() ||
		!gotDirectoryCreationParameters ||
		!gotDirectoryReaders ||
		!fileCreationParametersMessage.IsSet() {
		return PatchedTargetActionInputRootValue{}, evaluation.ErrMissingDependency
	}

	actionDefinition := action.Message.Definition
	if actionDefinition == nil {
		return PatchedTargetActionInputRootValue{}, errors.New("action definition missing")
	}

	var rootDirectory changeTrackingDirectory[TReference, TMetadata]
	loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
		context:                 ctx,
		directoryContentsReader: directoryReaders.DirectoryContents,
		leavesReader:            directoryReaders.Leaves,
	}

	// Add empty directories for the output directory of the current
	// package and configuration.
	components, err := getPackageOutputDirectoryComponents(
		model_core.Nested(id, id.Message.ConfigurationReference),
		targetLabel.GetCanonicalPackage(),
	)
	if err != nil {
		return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to get package output directory: %w", err)
	}
	outputDirectory := &rootDirectory
	for _, component := range components {
		outputDirectory, err = outputDirectory.getOrCreateDirectory(component)
		if err != nil {
			return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to create directory %#v: %w", component.String(), err)
		}
	}
	outputDirectory.unmodifiedDirectory = model_core.Nested(action, actionDefinition.InitialOutputDirectory)

	// Add input files.
	if err := addFilesToChangeTrackingDirectory(
		e,
		model_core.Nested(action, actionDefinition.Inputs),
		&rootDirectory,
		loadOptions,
	); err != nil {
		return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to add input files to input root: %w", err)
	}

	// Add tools.
	var errIter error
	for tool := range btree.AllLeaves(
		ctx,
		c.filesToRunProviderReader,
		model_core.Nested(action, actionDefinition.Tools),
		func(element model_core.Message[*model_analysis_pb.FilesToRunProvider, TReference]) (*model_core_pb.DecodableReference, error) {
			return element.Message.GetParent().GetReference(), nil
		},
		&errIter,
	) {
		toolLevel, ok := tool.Message.Level.(*model_analysis_pb.FilesToRunProvider_Leaf_)
		if !ok {
			return PatchedTargetActionInputRootValue{}, errors.New("not a valid leaf entry for tool")
		}
		toolLeaf := toolLevel.Leaf

		// Add the tool's executable to the input root.
		executable := model_core.Nested(tool, toolLeaf.Executable)
		if err := addFileToChangeTrackingDirectory(e, executable, &rootDirectory, loadOptions); err != nil {
			return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to add tool executable to input root: %w", err)
		}

		// Create the tool's runfiles directory.
		executablePath, err := model_starlark.FileGetPath(executable)
		if err != nil {
			return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to get path of tool executable: %w", err)
		}
		runfilesDirectoryResolver := changeTrackingDirectoryNewDirectoryResolver[TReference, TMetadata]{
			loadOptions: loadOptions,
			stack:       util.NewNonEmptyStack(&rootDirectory),
		}
		if err := path.Resolve(path.UNIXFormat.NewParser(executablePath+".runfiles"), &runfilesDirectoryResolver); err != nil {
			return PatchedTargetActionInputRootValue{}, fmt.Errorf("failed to create runfiles directory for tool with path %#v: %w", executablePath, err)
		}

		if len(toolLeaf.RunfilesFiles) > 0 {
			return PatchedTargetActionInputRootValue{}, errors.New("TODO: add runfiles files to the input root")
		}
		if len(toolLeaf.RunfilesSymlinks) > 0 {
			return PatchedTargetActionInputRootValue{}, errors.New("TODO: add runfiles symlinks to the input root")
		}
		if len(toolLeaf.RunfilesRootSymlinks) > 0 {
			return PatchedTargetActionInputRootValue{}, errors.New("TODO: add runfiles root symlinks to the input root")
		}
	}
	if errIter != nil {
		return PatchedTargetActionInputRootValue{}, errIter
	}

	group, groupCtx := errgroup.WithContext(ctx)
	var createdRootDirectory model_filesystem.CreatedDirectory[TMetadata]
	group.Go(func() error {
		return model_filesystem.CreateDirectoryMerkleTree[TMetadata, TMetadata](
			groupCtx,
			semaphore.NewWeighted(1),
			group,
			directoryCreationParameters,
			&capturableChangeTrackingDirectory[TReference, TMetadata]{
				options: &capturableChangeTrackingDirectoryOptions[TReference, TMetadata]{
					context:                 ctx,
					directoryContentsReader: directoryReaders.DirectoryContents,
					objectCapturer:          e,
				},
				directory: &rootDirectory,
			},
			model_filesystem.NewSimpleDirectoryMerkleTreeCapturer[TMetadata](e),
			&createdRootDirectory,
		)
	})
	if err := group.Wait(); err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}

	rootDirectoryObject, err := model_core.MarshalAndEncodePatchedMessage(
		createdRootDirectory.Message,
		c.getReferenceFormat(),
		directoryCreationParameters.GetEncoder(),
	)
	if err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetActionInputRoot_Value{
			InputRootReference: patcher.CaptureAndAddDecodableReference(rootDirectoryObject, e),
		},
		model_core.MapReferenceMetadataToWalkers(patcher),
	), nil
}
