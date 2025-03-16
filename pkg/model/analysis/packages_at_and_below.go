package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
)

type getPackageDirectoryEnvironment[TReference any] interface {
	GetRepoValue(*model_analysis_pb.Repo_Key) model_core.Message[*model_analysis_pb.Repo_Value, TReference]
}

func (c *baseComputer[TReference, TMetadata]) getPackageDirectory(
	ctx context.Context,
	e getPackageDirectoryEnvironment[TReference],
	directoryReader model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Directory, TReference]],
	canonicalPackageStr string,
) (model_core.Message[*model_filesystem_pb.Directory, TReference], error) {
	canonicalPackage, err := label.NewCanonicalPackage(canonicalPackageStr)
	if err != nil {
		return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, errors.New("invalid base package")
	}

	repoValue := e.GetRepoValue(&model_analysis_pb.Repo_Key{
		CanonicalRepo: canonicalPackage.GetCanonicalRepo().String(),
	})
	if !repoValue.IsSet() {
		return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, evaluation.ErrMissingDependency
	}

	// Obtain the root directory of the repo.
	baseDirectory, err := model_parser.Dereference(ctx, directoryReader, model_core.NewNestedMessage(repoValue, repoValue.Message.RootDirectoryReference.GetReference()))
	if err != nil {
		return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, err
	}

	// Traverse into the directory belonging to the package path.
	for component := range strings.FieldsFuncSeq(
		canonicalPackage.GetPackagePath(),
		func(r rune) bool { return r == '/' },
	) {
		directories := baseDirectory.Message.Directories
		directoryIndex, ok := sort.Find(
			len(directories),
			func(i int) int { return strings.Compare(component, directories[i].Name) },
		)
		if !ok {
			// Base package does not exist.
			return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, nil
		}
		baseDirectory, err = model_filesystem.DirectoryNodeGetContents(ctx, directoryReader, model_core.NewNestedMessage(baseDirectory, directories[directoryIndex]))
		if err != nil {
			return model_core.Message[*model_filesystem_pb.Directory, TReference]{}, err
		}
	}
	return baseDirectory, nil
}

func (c *baseComputer[TReference, TMetadata]) ComputePackagesAtAndBelowValue(ctx context.Context, key *model_analysis_pb.PackagesAtAndBelow_Key, e PackagesAtAndBelowEnvironment[TReference, TMetadata]) (PatchedPackagesAtAndBelowValue, error) {
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	if !gotDirectoryReaders {
		return PatchedPackagesAtAndBelowValue{}, evaluation.ErrMissingDependency
	}

	packageDirectory, err := c.getPackageDirectory(ctx, e, directoryReaders.Directory, key.BasePackage)
	if err != nil {
		return PatchedPackagesAtAndBelowValue{}, err
	}
	if !packageDirectory.IsSet() {
		return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.PackagesAtAndBelow_Value{}), nil
	}

	// Find packages at and below the base package.
	checker := packageExistenceChecker[TReference]{
		context:         ctx,
		directoryReader: directoryReaders.Directory,
		leavesReader:    directoryReaders.Leaves,
	}
	packageAtBasePackage, err := checker.directoryIsPackage(packageDirectory)
	if err != nil {
		return PatchedPackagesAtAndBelowValue{}, err
	}
	if err := checker.findPackagesBelow(packageDirectory, nil); err != nil {
		return PatchedPackagesAtAndBelowValue{}, err
	}

	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.PackagesAtAndBelow_Value{
		PackageAtBasePackage:     packageAtBasePackage,
		PackagesBelowBasePackage: checker.packagesBelowBasePackage,
	}), nil
}

type packageExistenceChecker[TReference any] struct {
	context                  context.Context
	directoryReader          model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Directory, TReference]]
	leavesReader             model_parser.ParsedObjectReader[TReference, model_core.Message[*model_filesystem_pb.Leaves, TReference]]
	packagesBelowBasePackage []string
}

func (pec *packageExistenceChecker[TReference]) directoryIsPackage(d model_core.Message[*model_filesystem_pb.Directory, TReference]) (bool, error) {
	leaves, err := model_filesystem.DirectoryGetLeaves(pec.context, pec.leavesReader, d)
	if err != nil {
		return false, err
	}

	// TODO: Should we also consider symlinks having such names?
	files := leaves.Message.Files
	for _, filename := range buildDotBazelTargetNames {
		filenameStr := filename.String()
		if _, ok := sort.Find(
			len(files),
			func(i int) int { return strings.Compare(filenameStr, files[i].Name) },
		); ok {
			// Current directory is a package.
			return true, nil
		}
	}
	return false, nil
}

func (pec *packageExistenceChecker[TReference]) findPackagesBelow(d model_core.Message[*model_filesystem_pb.Directory, TReference], dTrace *path.Trace) error {
	for _, entry := range d.Message.Directories {
		name, ok := path.NewComponent(entry.Name)
		if !ok {
			return fmt.Errorf("invalid directory name %#v in directory %#v", entry.Name, dTrace.GetUNIXString())
		}
		childTrace := dTrace.Append(name)

		childDirectory, err := model_filesystem.DirectoryNodeGetContents(pec.context, pec.directoryReader, model_core.NewNestedMessage(d, entry))
		if err != nil {
			return fmt.Errorf("failed to get contents of directory %#v: %w", childTrace.GetUNIXString(), err)
		}

		directoryIsPackage, err := pec.directoryIsPackage(childDirectory)
		if err != nil {
			return fmt.Errorf("failed to determine whether directory %#v is a package: %w", dTrace.GetUNIXString(), err)
		}
		if directoryIsPackage {
			pec.packagesBelowBasePackage = append(pec.packagesBelowBasePackage, childTrace.GetUNIXString())
		} else {
			// Not a package. Find packages below.
			if err := pec.findPackagesBelow(childDirectory, childTrace); err != nil {
				return err
			}
		}
	}
	return nil
}
