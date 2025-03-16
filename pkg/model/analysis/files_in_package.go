package analysis

import (
	"context"
	"errors"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

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
			nil, // TODO: Directory!
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
