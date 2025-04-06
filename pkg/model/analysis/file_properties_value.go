package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// reposFilePropertiesResolver resolves the properties of a file
// contained in a repo. All repos are placed in a fictive root
// directory, which allows symbolic links with targets of shape
// "../${repo}/${file}" to resolve properly.
type reposFilePropertiesResolver[TReference object.BasicReference, TMetadata any] struct {
	context          context.Context
	directoryReaders *DirectoryReaders[TReference]
	environment      FilePropertiesEnvironment[TReference, TMetadata]

	currentRepo *model_filesystem.DirectoryComponentWalker[TReference]
}

var _ path.ComponentWalker = (*reposFilePropertiesResolver[object.LocalReference, model_core.ReferenceMetadata])(nil)

func (r *reposFilePropertiesResolver[TReference, TMetadata]) handleRepoOnUp() (path.ComponentWalker, error) {
	r.currentRepo = nil
	return r, nil
}

func (r *reposFilePropertiesResolver[TReference, TMetadata]) setCurrentRepo(name string) error {
	repoValue := r.environment.GetRepoValue(&model_analysis_pb.Repo_Key{
		CanonicalRepo: name,
	})
	if !repoValue.IsSet() {
		return evaluation.ErrMissingDependency
	}

	r.currentRepo = model_filesystem.NewDirectoryComponentWalker[TReference](
		r.context,
		r.directoryReaders.Directory,
		r.directoryReaders.Leaves,
		r.handleRepoOnUp,
		model_core.Nested(repoValue, repoValue.Message.RootDirectoryReference.GetReference()),
		nil,
	)
	return nil
}

func (r *reposFilePropertiesResolver[TReference, TMetadata]) OnDirectory(name path.Component) (path.GotDirectoryOrSymlink, error) {
	if err := r.setCurrentRepo(name.String()); err != nil {
		return nil, err
	}
	return path.GotDirectory{
		Child:        r.currentRepo,
		IsReversible: true,
	}, nil
}

func (r *reposFilePropertiesResolver[TReference, TMetadata]) OnTerminal(name path.Component) (*path.GotSymlink, error) {
	return path.OnTerminalViaOnDirectory(r, name)
}

func (r *reposFilePropertiesResolver[TReference, TMetadata]) OnUp() (path.ComponentWalker, error) {
	return nil, errors.New("path escapes repositories directory")
}

func (c *baseComputer[TReference, TMetadata]) ComputeFilePropertiesValue(ctx context.Context, key *model_analysis_pb.FileProperties_Key, e FilePropertiesEnvironment[TReference, TMetadata]) (PatchedFilePropertiesValue, error) {
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	if !gotDirectoryReaders {
		return PatchedFilePropertiesValue{}, evaluation.ErrMissingDependency
	}

	resolver := reposFilePropertiesResolver[TReference, TMetadata]{
		context:          ctx,
		directoryReaders: directoryReaders,
		environment:      e,
	}
	if err := resolver.setCurrentRepo(key.CanonicalRepo); err != nil {
		return PatchedFilePropertiesValue{}, fmt.Errorf("failed to resolve canonical repo directory: %w", err)
	}

	if err := path.Resolve(
		path.UNIXFormat.NewParser(key.Path),
		path.NewLoopDetectingScopeWalker(path.NewRelativeScopeWalker(resolver.currentRepo)),
	); err != nil {
		if status.Code(err) == codes.NotFound {
			return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_analysis_pb.FileProperties_Value{},
			), nil
		}
		return PatchedFilePropertiesValue{}, fmt.Errorf("failed to resolve path: %w", err)
	}

	if resolver.currentRepo == nil {
		return PatchedFilePropertiesValue{}, errors.New("path resolves to a location outside any repo")
	}
	fileProperties := resolver.currentRepo.GetCurrentFileProperties()
	if !fileProperties.IsSet() {
		return PatchedFilePropertiesValue{}, errors.New("path resolves to a directory")
	}

	patchedFileProperties := model_core.Patch(e, fileProperties)
	return model_core.NewPatchedMessage(
		&model_analysis_pb.FileProperties_Value{
			Exists: patchedFileProperties.Message,
		},
		model_core.MapReferenceMetadataToWalkers(patchedFileProperties.Patcher),
	), nil
}
