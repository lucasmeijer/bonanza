package analysis

import (
	"context"
	"fmt"
	"sort"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/glob"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"
)

type globDirectoryWalker[TReference object.BasicReference, TMetadata BaseComputerReferenceMetadata] struct {
	context            context.Context
	computer           *baseComputer[TReference, TMetadata]
	directoryReaders   *DirectoryReaders[TReference]
	includeDirectories bool
	matchedPaths       []string
}

func (w *globDirectoryWalker[TReference, TMetadata]) walkDirectory(d model_core.Message[*model_filesystem_pb.Directory, TReference], dPath *path.Trace, dMatcher *glob.Matcher) error {
	var childMatcher glob.Matcher

MatchDirectories:
	for _, entry := range d.Message.Directories {
		// Check whether the directory itself is a match.
		name, ok := path.NewComponent(entry.Name)
		if !ok {
			return fmt.Errorf("invalid name for directory %#v in directory %#v", entry.Name, dPath.GetUNIXString())
		}
		childMatcher.CopyFrom(dMatcher)
		for _, r := range name.String() {
			if !childMatcher.WriteRune(r) {
				continue MatchDirectories
			}
		}
		childPath := dPath.Append(name)
		if w.includeDirectories && childMatcher.IsMatch() {
			w.matchedPaths = append(w.matchedPaths, childPath.GetUNIXString())
		}

		// Traverse into the directory if the still reachable
		// part of the NFA contains positive end states.
		if childMatcher.WriteRune('/') {
			childDirectory, err := model_filesystem.DirectoryNodeGetContents(
				w.context,
				w.directoryReaders.Directory,
				model_core.NewNestedMessage(d, entry),
			)
			if err != nil {
				return fmt.Errorf("failed to get directory %#v", childPath.GetUNIXString())
			}
			if err := w.walkDirectory(childDirectory, childPath, &childMatcher); err != nil {
				return err
			}
		}
	}

	leaves, err := model_filesystem.DirectoryGetLeaves(
		w.context,
		w.directoryReaders.Leaves,
		d,
	)
	if err != nil {
		return fmt.Errorf("failed to get leaves for directory %#v", dPath.GetUNIXString())
	}

MatchFiles:
	for _, entry := range leaves.Message.Files {
		name, ok := path.NewComponent(entry.Name)
		if !ok {
			return fmt.Errorf("invalid name for file %#v in directory %#v", entry.Name, dPath.GetUNIXString())
		}
		childMatcher.CopyFrom(dMatcher)
		for _, r := range name.String() {
			if !childMatcher.WriteRune(r) {
				continue MatchFiles
			}
		}
		if childMatcher.IsMatch() {
			w.matchedPaths = append(w.matchedPaths, dPath.Append(name).GetUNIXString())
		}
	}

	// TODO: Should we also consider symbolic links?
	return nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeGlobValue(ctx context.Context, key *model_analysis_pb.Glob_Key, e GlobEnvironment[TReference, TMetadata]) (PatchedGlobValue, error) {
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	filesInPackageValue := e.GetFilesInPackageValue(&model_analysis_pb.FilesInPackage_Key{
		Package: key.Package,
	})
	if !gotDirectoryReaders || !filesInPackageValue.IsSet() {
		return PatchedGlobValue{}, evaluation.ErrMissingDependency
	}

	nfa, err := glob.NewNFAFromBytes(key.Pattern)
	if err != nil {
		return PatchedGlobValue{}, fmt.Errorf("invalid pattern: %w", err)
	}
	var matcher glob.Matcher
	matcher.Initialize(nfa)

	w := globDirectoryWalker[TReference, TMetadata]{
		context:            ctx,
		computer:           c,
		directoryReaders:   directoryReaders,
		includeDirectories: key.IncludeDirectories,
	}
	if err := w.walkDirectory(
		model_core.NewNestedMessage(filesInPackageValue, filesInPackageValue.Message.Directory),
		nil,
		&matcher,
	); err != nil {
		return PatchedGlobValue{}, nil
	}

	// Bazel's implementation of glob() sorts results
	// alphabetically, with "/" being treated like an ordinary
	// character.
	sort.Strings(w.matchedPaths)

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
		&model_analysis_pb.Glob_Value{
			MatchedPaths: w.matchedPaths,
		},
	), nil
}
