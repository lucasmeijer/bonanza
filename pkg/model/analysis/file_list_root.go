package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"
)

type getRootForFilesEnvironment[TReference, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]

	GetFileListRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileListRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileListRoot_Value, TReference]
	GetFileRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileRoot_Value, TReference]
}

func getRootForFiles[TReference object.BasicReference, TMetadata model_core.WalkableReferenceMetadata](e getRootForFilesEnvironment[TReference, TMetadata], fileList model_core.Message[[]*model_starlark_pb.List_Element, TReference]) (model_core.PatchedMessage[*model_filesystem_pb.Directory, TMetadata], error) {
	missingDependencies := false
	for i, element := range fileList.Message {
		var root model_core.Message[*model_filesystem_pb.Directory, TReference]
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
				return model_core.PatchedMessage[*model_filesystem_pb.Directory, TMetadata]{}, fmt.Errorf("element at index %d is not a file", i)
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
			return model_core.PatchedMessage[*model_filesystem_pb.Directory, TMetadata]{}, errors.New("invalid list level type")
		}

		// Optimize the case when this function is called
		// against a single element list. Return the resulting
		// root without any merging.
		if len(fileList.Message) == 1 {
			return model_core.Patch(e, root), nil
		}
	}
	if missingDependencies {
		return model_core.PatchedMessage[*model_filesystem_pb.Directory, TMetadata]{}, evaluation.ErrMissingDependency
	}
	return model_core.PatchedMessage[*model_filesystem_pb.Directory, TMetadata]{}, errors.New("TODO: Implement!")
}

func (c *baseComputer[TReference, TMetadata]) ComputeFileListRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.FileListRoot_Key, TReference], e FileListRootEnvironment[TReference, TMetadata]) (PatchedFileListRootValue, error) {
	filesList, err := model_parser.Dereference(ctx, c.valueReaders.List, model_core.Nested(key, key.Message.ListReference))
	if err != nil {
		return PatchedFileListRootValue{}, err
	}

	root, err := getRootForFiles(e, filesList)
	if err != nil {
		return PatchedFileListRootValue{}, err
	}

	return model_core.NewPatchedMessage(
		&model_analysis_pb.FileListRoot_Value{
			RootDirectory: root.Message,
		},
		model_core.MapReferenceMetadataToWalkers(root.Patcher),
	), nil
}
