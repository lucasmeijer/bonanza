package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type addFilesToChangeTrackingDirectoryEnvironment[TReference, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]

	GetFileListRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileListRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileListRoot_Value, TReference]
	GetFileRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileRoot_Value, TReference]
}

func addFilesToChangeTrackingDirectory[TReference object.BasicReference, TMetadata model_core.WalkableReferenceMetadata](
	e addFilesToChangeTrackingDirectoryEnvironment[TReference, TMetadata],
	fileList model_core.Message[[]*model_starlark_pb.List_Element, TReference],
	out *changeTrackingDirectory[TReference, TMetadata],
	loadOptions *changeTrackingDirectoryLoadOptions[TReference],
) error {
	missingDependencies := false
	for i, element := range fileList.Message {
		var root model_core.Message[*model_filesystem_pb.DirectoryContents, TReference]
		switch level := element.Level.(type) {
		case *model_starlark_pb.List_Element_Parent_:
			patchedReference := model_core.Patch(e, model_core.Nested(fileList, level.Parent.Reference))
			v := e.GetFileListRootValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.FileListRoot_Key{
						ListReference: patchedReference.Message,
					},
					model_core.MapReferenceMetadataToWalkers(patchedReference.Patcher),
				),
			)
			if !v.IsSet() {
				missingDependencies = true
				continue
			}
			root = model_core.Nested(v, v.Message.RootDirectory)
		case *model_starlark_pb.List_Element_Leaf:
			file, ok := level.Leaf.Kind.(*model_starlark_pb.Value_File)
			if !ok {
				return fmt.Errorf("element at index %d is not a file", i)
			}
			patchedFile := model_core.Patch(e, model_core.Nested(fileList, file.File))
			v := e.GetFileRootValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.FileRoot_Key{
						File: patchedFile.Message,
					},
					model_core.MapReferenceMetadataToWalkers(patchedFile.Patcher),
				),
			)
			if !v.IsSet() {
				missingDependencies = true
				continue
			}
			root = model_core.Nested(v, v.Message.RootDirectory)
		default:
			return errors.New("invalid list level type")
		}
		if err := out.mergeContents(root, loadOptions); err != nil {
			return fmt.Errorf("list element at index %d: %w", i, err)
		}
	}
	if missingDependencies {
		return evaluation.ErrMissingDependency
	}
	return nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeFileListRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.FileListRoot_Key, TReference], e FileListRootEnvironment[TReference, TMetadata]) (PatchedFileListRootValue, error) {
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	if !gotDirectoryCreationParameters || !gotDirectoryReaders {
		return PatchedFileListRootValue{}, evaluation.ErrMissingDependency
	}

	filesList, err := model_parser.Dereference(ctx, c.valueReaders.List, model_core.Nested(key, key.Message.ListReference))
	if err != nil {
		return PatchedFileListRootValue{}, err
	}

	var rootDirectory changeTrackingDirectory[TReference, TMetadata]
	if err := addFilesToChangeTrackingDirectory(
		e,
		filesList,
		&rootDirectory,
		&changeTrackingDirectoryLoadOptions[TReference]{
			context:                 ctx,
			directoryContentsReader: directoryReaders.DirectoryContents,
			leavesReader:            directoryReaders.Leaves,
		},
	); err != nil {
		return PatchedFileListRootValue{}, err
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
		return PatchedFileListRootValue{}, err
	}

	return model_core.NewPatchedMessage(
		&model_analysis_pb.FileListRoot_Value{
			RootDirectory: createdRootDirectory.Message.Message,
		},
		model_core.MapReferenceMetadataToWalkers(createdRootDirectory.Message.Patcher),
	), nil
}
