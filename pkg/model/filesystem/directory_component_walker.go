package filesystem

import (
	"context"
	"sort"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DirectoryComponentWalker[TReference object.BasicReference] struct {
	// Constant fields.
	context         context.Context
	directoryReader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Directory, TReference]]
	leavesReader    model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Leaves, TReference]]
	onUpHandler     func() (path.ComponentWalker, error)

	// Variable fields.
	currentDirectoryReference model_core.Message[*model_core_pb.Reference, TReference]
	stack                     []model_core.Message[*model_filesystem_pb.Directory, TReference]
	fileProperties            model_core.Message[*model_filesystem_pb.FileProperties, TReference]
}

var _ path.ComponentWalker = (*DirectoryComponentWalker[object.BasicReference])(nil)

func NewDirectoryComponentWalker[TReference object.BasicReference](
	ctx context.Context,
	directoryReader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Directory, TReference]],
	leavesReader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Leaves, TReference]],
	onUpHandler func() (path.ComponentWalker, error),
	currentDirectoryReference model_core.Message[*model_core_pb.Reference, TReference],
	stack []model_core.Message[*model_filesystem_pb.Directory, TReference],
) *DirectoryComponentWalker[TReference] {
	return &DirectoryComponentWalker[TReference]{
		context:         ctx,
		directoryReader: directoryReader,
		leavesReader:    leavesReader,
		onUpHandler:     onUpHandler,

		currentDirectoryReference: currentDirectoryReference,
		stack:                     stack,
	}
}

func (cw *DirectoryComponentWalker[TReference]) dereferenceCurrentDirectory() error {
	if cw.currentDirectoryReference.IsSet() {
		d, err := model_parser.Dereference(cw.context, cw.directoryReader, cw.currentDirectoryReference)
		if err != nil {
			return err
		}
		cw.stack = append(cw.stack, d)
		cw.currentDirectoryReference.Clear()
	}
	return nil
}

func (cw *DirectoryComponentWalker[TReference]) OnDirectory(name path.Component) (path.GotDirectoryOrSymlink, error) {
	if err := cw.dereferenceCurrentDirectory(); err != nil {
		return nil, err
	}

	d := cw.stack[len(cw.stack)-1]
	n := name.String()
	directories := d.Message.Directories
	if i, ok := sort.Find(
		len(directories),
		func(i int) int { return strings.Compare(n, directories[i].Name) },
	); ok {
		switch contents := directories[i].Contents.(type) {
		case *model_filesystem_pb.DirectoryNode_ContentsExternal:
			cw.currentDirectoryReference = model_core.Nested(d, contents.ContentsExternal.Reference)
		case *model_filesystem_pb.DirectoryNode_ContentsInline:
			cw.stack = append(cw.stack, model_core.Nested(d, contents.ContentsInline))
		default:
			return nil, status.Error(codes.InvalidArgument, "Unknown directory contents type")
		}
		return path.GotDirectory{
			Child:        cw,
			IsReversible: true,
		}, nil
	}

	leaves, err := DirectoryGetLeaves(cw.context, cw.leavesReader, d)
	if err != nil {
		return nil, err
	}

	files := leaves.Message.Files
	if _, ok := sort.Find(
		len(files),
		func(i int) int { return strings.Compare(n, files[i].Name) },
	); ok {
		return nil, status.Error(codes.InvalidArgument, "Path resolves to a regular file, while a directory was expected")
	}

	symlinks := leaves.Message.Symlinks
	if i, ok := sort.Find(
		len(symlinks),
		func(i int) int { return strings.Compare(n, symlinks[i].Name) },
	); ok {
		return path.GotSymlink{
			Parent: path.NewRelativeScopeWalker(cw),
			Target: path.UNIXFormat.NewParser(symlinks[i].Target),
		}, nil
	}

	return nil, status.Error(codes.NotFound, "Path does not exist")
}

func (cw *DirectoryComponentWalker[TReference]) OnTerminal(name path.Component) (*path.GotSymlink, error) {
	if err := cw.dereferenceCurrentDirectory(); err != nil {
		return nil, err
	}

	d := cw.stack[len(cw.stack)-1]
	n := name.String()
	directories := d.Message.Directories
	if _, ok := sort.Find(
		len(directories),
		func(i int) int { return strings.Compare(n, directories[i].Name) },
	); ok {
		return nil, nil
	}

	leaves, err := DirectoryGetLeaves(cw.context, cw.leavesReader, d)
	if err != nil {
		return nil, err
	}

	files := leaves.Message.Files
	if i, ok := sort.Find(
		len(files),
		func(i int) int { return strings.Compare(n, files[i].Name) },
	); ok {
		properties := files[i].Properties
		if properties == nil {
			return nil, status.Error(codes.InvalidArgument, "Path resolves to file that does not have any properties")
		}
		cw.fileProperties = model_core.Nested(leaves, properties)
		return nil, nil
	}

	symlinks := leaves.Message.Symlinks
	if i, ok := sort.Find(
		len(symlinks),
		func(i int) int { return strings.Compare(n, symlinks[i].Name) },
	); ok {
		return &path.GotSymlink{
			Parent: path.NewRelativeScopeWalker(cw),
			Target: path.UNIXFormat.NewParser(symlinks[i].Target),
		}, nil
	}

	return nil, status.Error(codes.NotFound, "Path does not exist")
}

func (cw *DirectoryComponentWalker[TReference]) OnUp() (path.ComponentWalker, error) {
	if cw.currentDirectoryReference.IsSet() {
		cw.currentDirectoryReference.Clear()
	} else if len(cw.stack) == 1 {
		return cw.onUpHandler()
	} else {
		cw.stack = cw.stack[:len(cw.stack)-1]
	}
	return cw, nil
}

func (cw *DirectoryComponentWalker[TReference]) GetCurrentFileProperties() model_core.Message[*model_filesystem_pb.FileProperties, TReference] {
	return cw.fileProperties
}

// DirectoryGetLeaves is a helper function for obtaining the leaves
// contained in a directory.
func DirectoryGetLeaves[TReference any](
	ctx context.Context,
	reader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Leaves, TReference]],
	directory model_core.Message[*model_filesystem_pb.Directory, TReference],
) (model_core.Message[*model_filesystem_pb.Leaves, TReference], error) {
	switch leaves := directory.Message.Leaves.(type) {
	case *model_filesystem_pb.Directory_LeavesExternal:
		return model_parser.Dereference(ctx, reader, model_core.Nested(directory, leaves.LeavesExternal.Reference))
	case *model_filesystem_pb.Directory_LeavesInline:
		return model_core.Nested(directory, leaves.LeavesInline), nil
	default:
		return model_core.Message[*model_filesystem_pb.Leaves, TReference]{}, status.Error(codes.InvalidArgument, "Directory has no leaves")
	}
}

// DirectoryNodeGetContents is a helper function for obtaining the
// contents of a directory node.
func DirectoryNodeGetContents[TReference any](
	ctx context.Context,
	reader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Directory, TReference]],
	directoryNode model_core.Message[*model_filesystem_pb.DirectoryNode, TReference],
) (model_core.Message[*model_filesystem_pb.Directory, TReference], error) {
	switch contents := directoryNode.Message.Contents.(type) {
	case *model_filesystem_pb.DirectoryNode_ContentsExternal:
		return model_parser.Dereference(ctx, reader, model_core.Nested(directoryNode, contents.ContentsExternal.Reference))
	case *model_filesystem_pb.DirectoryNode_ContentsInline:
		return model_core.Nested(directoryNode, contents.ContentsInline), nil
	default:
		return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, status.Error(codes.InvalidArgument, "Directory node has no contents")
	}
}
