package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem"
	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type currentPackageLimitingDirectoryOptions[TReference any] struct {
	context          context.Context
	directoryReaders *DirectoryReaders[TReference]
}

// currentPackageLimitingDirectory is an implementation of
// CapturableDirectory that suppresses child directories that are not
// part of the current package. Furthermore, it strips the contents of
// all files. This leaves the only data that is necessary for making
// features like glob() work.
type currentPackageLimitingDirectory[TReference any, TDirectory, TFile model_core.ReferenceMetadata] struct {
	options   *currentPackageLimitingDirectoryOptions[TReference]
	directory model_core.Message[*model_filesystem_pb.Directory, TReference]
}

func (d *currentPackageLimitingDirectory[TReference, TDirectory, TFile]) Close() error {
	*d = currentPackageLimitingDirectory[TReference, TDirectory, TFile]{}
	return nil
}

func (d *currentPackageLimitingDirectory[TReference, TDirectory, TFile]) ReadDir() ([]filesystem.FileInfo, error) {
	leaves, err := model_filesystem.DirectoryGetLeaves(
		d.options.context,
		d.options.directoryReaders.Leaves,
		d.directory,
	)
	if err != nil {
		return nil, err
	}

	// Iterate over all children in sorted order. As the individual
	// lists of directories, files and symlinks are already sorted,
	// we merely need to merge them.
	directories := d.directory.Message.Directories
	files := leaves.Message.Files
	symlinks := leaves.Message.Symlinks
	fileInfos := make([]filesystem.FileInfo, 0, len(directories)+len(files)+len(symlinks))
	for len(directories) > 0 || len(files) > 0 || len(symlinks) > 0 {
		if len(directories) > 0 {
			entry := directories[0]
			if (len(files) == 0 || strings.Compare(entry.Name, files[0].Name) < 0) &&
				(len(symlinks) == 0 || strings.Compare(entry.Name, symlinks[0].Name) < 0) {
				// Report directory if it is not a package.
				childDirectory, err := model_filesystem.DirectoryNodeGetContents(
					d.options.context,
					d.options.directoryReaders.Directory,
					model_core.NewNestedMessage(d.directory, entry),
				)
				if err != nil {
					return nil, fmt.Errorf("failed to get contents for directory %#v: %w", entry.Name, err)
				}

				isPackage, err := directoryIsPackage(
					d.options.context,
					d.options.directoryReaders.Leaves,
					childDirectory,
				)
				if err != nil {
					return nil, err
				}
				if !isPackage {
					name, ok := path.NewComponent(entry.Name)
					if !ok {
						return nil, fmt.Errorf("invalid name for directory %#v", entry.Name)
					}
					fileInfos = append(fileInfos, filesystem.NewFileInfo(name, filesystem.FileTypeDirectory, false))
				}
				directories = directories[1:]
				continue
			}
		}

		if len(files) > 0 {
			entry := files[0]
			if len(symlinks) == 0 || strings.Compare(entry.Name, symlinks[0].Name) < 0 {
				// Report regular file.
				name, ok := path.NewComponent(entry.Name)
				if !ok {
					return nil, fmt.Errorf("invalid name for file %#v", entry.Name)
				}
				fileInfos = append(fileInfos, filesystem.NewFileInfo(name, filesystem.FileTypeRegularFile, entry.Properties.GetIsExecutable()))
				files = files[1:]
				continue
			}
		}

		// Report symbolic link.
		entry := symlinks[0]
		name, ok := path.NewComponent(entry.Name)
		if !ok {
			return nil, fmt.Errorf("invalid name for symbolic link %#v", entry.Name)
		}
		fileInfos = append(fileInfos, filesystem.NewFileInfo(name, filesystem.FileTypeSymlink, false))
		symlinks = symlinks[1:]
	}
	return fileInfos, nil
}

func (d *currentPackageLimitingDirectory[TReference, TDirectory, TFile]) Readlink(name path.Component) (path.Parser, error) {
	leaves, err := model_filesystem.DirectoryGetLeaves(
		d.options.context,
		d.options.directoryReaders.Leaves,
		d.directory,
	)
	if err != nil {
		return nil, err
	}

	symlinks := leaves.Message.Symlinks
	nameStr := name.String()
	index, ok := sort.Find(
		len(symlinks),
		func(i int) int { return strings.Compare(nameStr, symlinks[i].Name) },
	)
	if !ok {
		panic("attempted to read a symbolic link that was not reported by ReadDir()")
	}
	return path.UNIXFormat.NewParser(symlinks[index].Target), nil
}

func (d *currentPackageLimitingDirectory[TReference, TDirectory, TFile]) EnterCapturableDirectory(name path.Component) (*model_filesystem.CreatedDirectory[TDirectory], model_filesystem.CapturableDirectory[TDirectory, TFile], error) {
	directories := d.directory.Message.Directories
	nameStr := name.String()
	index, ok := sort.Find(
		len(directories),
		func(i int) int { return strings.Compare(nameStr, directories[i].Name) },
	)
	if !ok {
		panic("attempted to enter a directory that was not reported by ReadDir()")
	}
	childDirectory, err := model_filesystem.DirectoryNodeGetContents(
		d.options.context,
		d.options.directoryReaders.Directory,
		model_core.NewNestedMessage(d.directory, directories[index]),
	)
	if err != nil {
		return nil, nil, err
	}
	return nil, &currentPackageLimitingDirectory[TReference, TDirectory, TFile]{
		options:   d.options,
		directory: childDirectory,
	}, nil
}

func (currentPackageLimitingDirectory[TReference, TDirectory, TFile]) OpenForFileMerkleTreeCreation(name path.Component) (model_filesystem.CapturableFile[TFile], error) {
	return emptyCapturableFile[TFile]{}, nil
}

type emptyCapturableFile[TMetadata model_core.ReferenceMetadata] struct{}

func (emptyCapturableFile[TMetadata]) CreateFileMerkleTree(ctx context.Context) (model_core.PatchedMessage[*model_filesystem_pb.FileContents, TMetadata], error) {
	return model_core.NewSimplePatchedMessage[TMetadata]((*model_filesystem_pb.FileContents)(nil)), nil
}
func (emptyCapturableFile[TMetadata]) Discard() {}

func (c *baseComputer[TReference, TMetadata]) ComputeFilesInPackageValue(ctx context.Context, key *model_analysis_pb.FilesInPackage_Key, e FilesInPackageEnvironment[TReference, TMetadata]) (PatchedFilesInPackageValue, error) {
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	if !gotDirectoryCreationParameters || !gotDirectoryReaders {
		return PatchedFilesInPackageValue{}, evaluation.ErrMissingDependency
	}

	packageDirectory, err := c.getPackageDirectory(ctx, e, directoryReaders.Directory, key.Package)
	if err != nil {
		return PatchedFilesInPackageValue{}, err
	}
	if !packageDirectory.IsSet() {
		return PatchedFilesInPackageValue{}, errors.New("package directory does not exist")
	}

	var trimmedPackageDirectory model_filesystem.CreatedDirectory[TMetadata]
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return model_filesystem.CreateDirectoryMerkleTree[TMetadata, TMetadata](
			groupCtx,
			semaphore.NewWeighted(1),
			group,
			directoryCreationParameters,
			&currentPackageLimitingDirectory[TReference, TMetadata, TMetadata]{
				options: &currentPackageLimitingDirectoryOptions[TReference]{
					context:          ctx,
					directoryReaders: directoryReaders,
				},
				directory: packageDirectory,
			},
			nil, // TODO: Capturer!
			&trimmedPackageDirectory,
		)
	})
	if err := group.Wait(); err != nil {
		return PatchedFilesInPackageValue{}, err
	}

	return model_core.NewPatchedMessage(
		&model_analysis_pb.FilesInPackage_Value{
			Directory: trimmedPackageDirectory.Message.Message,
		},
		model_core.MapReferenceMetadataToWalkers(trimmedPackageDirectory.Message.Patcher),
	), nil
}
