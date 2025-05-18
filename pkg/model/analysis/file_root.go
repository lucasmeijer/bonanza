package analysis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem"
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
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/search"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type getStarlarkFilePropertiesEnvironment[TReference any, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]

	GetDirectoryReadersValue(key *model_analysis_pb.DirectoryReaders_Key) (*DirectoryReaders[TReference], bool)
	GetFilePropertiesValue(key *model_analysis_pb.FileProperties_Key) model_core.Message[*model_analysis_pb.FileProperties_Value, TReference]
	GetFileRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileRoot_Value, TReference]
}

func getStarlarkFileProperties[TReference object.BasicReference, TMetadata model_core.WalkableReferenceMetadata](ctx context.Context, e getStarlarkFilePropertiesEnvironment[TReference, TMetadata], f model_core.Message[*model_starlark_pb.File, TReference]) (model_core.Message[*model_filesystem_pb.FileProperties, TReference], error) {
	if f.Message == nil {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, errors.New("file not set")
	}

	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	patchedFile := model_core.Patch(e, f)
	targetOutput := e.GetFileRootValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.FileRoot_Key{
				File: patchedFile.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedFile.Patcher),
		),
	)
	if !gotDirectoryReaders || !targetOutput.IsSet() {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, evaluation.ErrMissingDependency
	}

	filePath, err := model_starlark.FileGetPath(f)
	if err != nil {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, err
	}
	componentWalker := model_filesystem.NewDirectoryComponentWalker[TReference](
		ctx,
		directoryReaders.DirectoryContents,
		directoryReaders.Leaves,
		func() (path.ComponentWalker, error) {
			return nil, errors.New("path resolution escapes input root")
		},
		model_core.Message[*model_core_pb.DecodableReference, TReference]{},
		[]model_core.Message[*model_filesystem_pb.DirectoryContents, TReference]{
			model_core.Nested(targetOutput, targetOutput.Message.RootDirectory),
		},
	)
	if err := path.Resolve(
		path.UNIXFormat.NewParser(filePath),
		path.NewLoopDetectingScopeWalker(path.NewRelativeScopeWalker(componentWalker)),
	); err != nil {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, fmt.Errorf("failed to resolve path: %w", err)
	}
	fileProperties := componentWalker.GetCurrentFileProperties()
	if !fileProperties.IsSet() {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, errors.New("target output is a directory")
	}
	return fileProperties, nil
}

func getPackageOutputDirectoryComponents[TReference object.BasicReference](configurationReference model_core.Message[*model_core_pb.DecodableReference, TReference], canonicalPackage label.CanonicalPackage) ([]path.Component, error) {
	// TODO: Add more utility functions to pkg/label, so that we
	// don't need to call path.MustNewComponent() from here.
	configurationComponent, err := model_starlark.ConfigurationReferenceToComponent(configurationReference)
	if err != nil {
		return nil, err
	}
	components := []path.Component{
		model_starlark.ComponentBazelOut,
		path.MustNewComponent(configurationComponent),
		model_starlark.ComponentBin,
		model_starlark.ComponentExternal,
		path.MustNewComponent(canonicalPackage.GetCanonicalRepo().String()),
	}
	for packageComponent := range strings.FieldsFuncSeq(canonicalPackage.GetPackagePath(), func(r rune) bool { return r == '/' }) {
		components = append(components, path.MustNewComponent(packageComponent))
	}
	return components, nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeFileRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.FileRoot_Key, TReference], e FileRootEnvironment[TReference, TMetadata]) (PatchedFileRootValue, error) {
	f := model_core.Nested(key, key.Message.File)
	if f.Message == nil {
		return PatchedFileRootValue{}, fmt.Errorf("no file provided")
	}
	fileLabel, err := label.NewCanonicalLabel(f.Message.Label)
	if err != nil {
		return PatchedFileRootValue{}, fmt.Errorf("invalid label: %w", err)
	}

	if o := f.Message.Owner; o != nil {
		targetName, err := label.NewTargetName(o.TargetName)
		if err != nil {
			return PatchedFileRootValue{}, fmt.Errorf("invalid target name: %w", err)
		}

		configurationReference := model_core.Nested(f, o.ConfigurationReference)
		targetLabel := fileLabel.GetCanonicalPackage().AppendTargetName(targetName)
		patchedConfigurationReference := model_core.Patch(e, configurationReference)
		configuredTarget := e.GetConfiguredTargetValue(
			model_core.NewPatchedMessage(
				&model_analysis_pb.ConfiguredTarget_Key{
					Label:                  targetLabel.String(),
					ConfigurationReference: patchedConfigurationReference.Message,
				},
				model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
			),
		)
		if !configuredTarget.IsSet() {
			return PatchedFileRootValue{}, evaluation.ErrMissingDependency
		}

		packageRelativePathStr := fileLabel.GetTargetName().String()
		output, err := btree.Find(
			ctx,
			c.configuredTargetOutputReader,
			model_core.Nested(configuredTarget, configuredTarget.Message.Outputs),
			func(entry model_core.Message[*model_analysis_pb.ConfiguredTarget_Value_Output, TReference]) (int, *model_core_pb.DecodableReference) {
				switch level := entry.Message.Level.(type) {
				case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_:
					return strings.Compare(packageRelativePathStr, level.Leaf.PackageRelativePath), nil
				case *model_analysis_pb.ConfiguredTarget_Value_Output_Parent_:
					return strings.Compare(packageRelativePathStr, level.Parent.FirstPackageRelativePath), level.Parent.Reference
				default:
					return 0, nil
				}
			},
		)
		if err != nil {
			return PatchedFileRootValue{}, err
		}
		if !output.IsSet() {
			return PatchedFileRootValue{}, errors.New("target does not yield an output with the provided name")
		}
		outputLeaf, ok := output.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_)
		if !ok {
			return PatchedFileRootValue{}, errors.New("unknown output level type")
		}

		switch source := outputLeaf.Leaf.Source.(type) {
		case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_ActionId:
			patchedConfigurationReference := model_core.Patch(e, configurationReference)
			targetActionResult := e.GetTargetActionResultValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.TargetActionResult_Key{
						Id: &model_analysis_pb.TargetActionId{
							Label:                  targetLabel.String(),
							ConfigurationReference: patchedConfigurationReference.Message,
							ActionId:               source.ActionId,
						},
					},
					model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
				),
			)
			if !targetActionResult.IsSet() {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			// TODO: We currently return the entire output
			// root of the action. We should trim it to only
			// contain the file or directory that was
			// requested.
			patchedOutputRoot := model_core.Patch(e, model_core.Nested(targetActionResult, targetActionResult.Message.OutputRoot))
			return model_core.NewPatchedMessage(
				&model_analysis_pb.FileRoot_Value{
					RootDirectory: patchedOutputRoot.Message,
				},
				model_core.MapReferenceMetadataToWalkers(patchedOutputRoot.Patcher),
			), nil
		case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_ExpandTemplate_:
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			fileCreationParameters, gotFileCreationParameters := e.GetFileCreationParametersObjectValue(&model_analysis_pb.FileCreationParametersObject_Key{})
			fileReader, gotFileReader := e.GetFileReaderValue(&model_analysis_pb.FileReader_Key{})
			if !gotDirectoryCreationParameters || !gotFileCreationParameters || !gotFileReader {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			// Look up template file.
			templateFileProperties, err := getStarlarkFileProperties(ctx, e, model_core.Nested(output, source.ExpandTemplate.Template))
			if err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("failed to file properties of template: %w", err)
			}
			templateContentsEntry, err := model_filesystem.NewFileContentsEntryFromProto(model_core.Nested(templateFileProperties, templateFileProperties.Message.Contents))
			if err != nil {
				return PatchedFileRootValue{}, err
			}

			// Create search and replacer for performing substitutions.
			substitutions := source.ExpandTemplate.Substitutions
			needles := make([][]byte, 0, len(substitutions))
			replacements := make([][]byte, 0, len(substitutions))
			for _, substitution := range substitutions {
				needles = append(needles, substitution.Needle)
				replacements = append(replacements, substitution.Replacement)
			}
			searchAndReplacer, err := search.NewMultiSearchAndReplacer(needles)
			if err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("invalid substitution keys: %w", err)
			}

			merkleTreeNodes, err := c.filePool.NewFile()
			if err != nil {
				return PatchedFileRootValue{}, err
			}
			defer func() {
				if merkleTreeNodes != nil {
					merkleTreeNodes.Close()
				}
			}()
			fileWritingObjectCapturer := model_core.NewFileWritingObjectCapturer(model_filesystem.NewSectionWriter(merkleTreeNodes))

			// Perform substitutions and create a new Merkle tree
			// for the resulting output file.
			pipeReader, pipeWriter := io.Pipe()
			var outputFileContents model_core.PatchedMessage[*model_filesystem_pb.FileContents, model_core.FileBackedObjectLocation]
			group, groupCtx := errgroup.WithContext(ctx)
			group.Go(func() error {
				err := searchAndReplacer.SearchAndReplace(
					pipeWriter,
					bufio.NewReader(fileReader.FileOpenRead(groupCtx, templateContentsEntry, 0)),
					replacements,
				)
				pipeWriter.CloseWithError(err)
				return err
			})
			group.Go(func() error {
				var err error
				outputFileContents, err = model_filesystem.CreateFileMerkleTree(
					groupCtx,
					fileCreationParameters,
					pipeReader,
					model_filesystem.NewSimpleFileMerkleTreeCapturer(fileWritingObjectCapturer),
				)
				pipeReader.CloseWithError(err)
				return err
			})
			if err := group.Wait(); err != nil {
				return PatchedFileRootValue{}, err
			}

			components, err := getPackageOutputDirectoryComponents(configurationReference, fileLabel.GetCanonicalPackage())
			if err != nil {
				return PatchedFileRootValue{}, err
			}

			// Place the output file in a directory structure.
			var createdDirectory model_filesystem.CreatedDirectory[model_core.FileBackedObjectLocation]
			group, groupCtx = errgroup.WithContext(ctx)
			group.Go(func() error {
				return model_filesystem.CreateDirectoryMerkleTree(
					groupCtx,
					semaphore.NewWeighted(1),
					group,
					directoryCreationParameters,
					&singleFileDirectory[model_core.FileBackedObjectLocation, model_core.FileBackedObjectLocation]{
						components:   append(components, fileLabel.GetTargetName().ToComponents()...),
						isExecutable: source.ExpandTemplate.IsExecutable,
						file:         model_filesystem.NewSimpleCapturableFile(outputFileContents),
					},
					model_filesystem.NewSimpleDirectoryMerkleTreeCapturer(fileWritingObjectCapturer),
					&createdDirectory,
				)
			})
			if err := group.Wait(); err != nil {
				return PatchedFileRootValue{}, err
			}

			// Flush the created Merkle tree to disk, so that it can
			// be read back during the uploading process.
			if err := fileWritingObjectCapturer.Flush(); err != nil {
				return PatchedFileRootValue{}, err
			}
			objectContentsWalkerFactory := model_core.NewFileReadingObjectContentsWalkerFactory(merkleTreeNodes)
			defer objectContentsWalkerFactory.Release()
			merkleTreeNodes = nil

			return model_core.NewPatchedMessage(
				&model_analysis_pb.FileRoot_Value{
					RootDirectory: createdDirectory.Message.Message,
				},
				model_core.MapReferenceMessagePatcherMetadata(
					createdDirectory.Message.Patcher,
					objectContentsWalkerFactory.CreateObjectContentsWalker,
				),
			), nil
		case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_StaticPackageDirectory:
			// Output file was already computed during configuration.
			// For example by calling ctx.actions.write() or
			// ctx.actions.symlink(target_path=...).
			//
			// Wrap the package directory to make it an input root.
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			if !gotDirectoryCreationParameters {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			components, err := getPackageOutputDirectoryComponents(configurationReference, fileLabel.GetCanonicalPackage())
			if err != nil {
				return PatchedFileRootValue{}, err
			}

			var createdDirectory model_filesystem.CreatedDirectory[TMetadata]
			group, groupCtx := errgroup.WithContext(ctx)
			group.Go(func() error {
				return model_filesystem.CreateDirectoryMerkleTree(
					groupCtx,
					semaphore.NewWeighted(1),
					group,
					directoryCreationParameters,
					&pathPrependingDirectory[TMetadata, TMetadata]{
						components: components,
						directory:  model_core.Patch(e, model_core.Nested(output, source.StaticPackageDirectory)),
					},
					model_filesystem.NewSimpleDirectoryMerkleTreeCapturer(e),
					&createdDirectory,
				)
			})
			if err := group.Wait(); err != nil {
				return PatchedFileRootValue{}, err
			}

			return model_core.NewPatchedMessage(
				&model_analysis_pb.FileRoot_Value{
					RootDirectory: createdDirectory.Message.Message,
				},
				model_core.MapReferenceMetadataToWalkers(createdDirectory.Message.Patcher),
			), nil
		case *model_analysis_pb.ConfiguredTarget_Value_Output_Leaf_Symlink:
			// Symlink to another file. Obtain the root of
			// the target and add a symlink to it.
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
			patchedSymlinkTargetFile := model_core.Patch(e, model_core.Nested(output, source.Symlink))
			symlinkTarget := e.GetFileRootValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.FileRoot_Key{
						File: patchedSymlinkTargetFile.Message,
					},
					model_core.MapReferenceMetadataToWalkers(patchedSymlinkTargetFile.Patcher),
				),
			)
			if !gotDirectoryCreationParameters || !gotDirectoryReaders || !symlinkTarget.IsSet() {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			var rootDirectory changeTrackingDirectory[TReference, TMetadata]
			loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
				context:                 ctx,
				directoryContentsReader: directoryReaders.DirectoryContents,
				leavesReader:            directoryReaders.Leaves,
			}
			if err := rootDirectory.setContents(
				model_core.Nested(symlinkTarget, symlinkTarget.Message.RootDirectory),
				loadOptions,
			); err != nil {
				return PatchedFileRootValue{}, err
			}

			symlinkPath, err := model_starlark.FileGetPath(f)
			if err != nil {
				return PatchedFileRootValue{}, err
			}
			r := &changeTrackingDirectoryNewFileResolver[TReference, TMetadata]{
				loadOptions: loadOptions,
				stack:       util.NewNonEmptyStack(&rootDirectory),
			}
			if err := path.Resolve(path.UNIXFormat.NewParser(symlinkPath), r); err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("cannot resolve %#v: %w", symlinkPath, err)
			}
			if r.TerminalName == nil {
				return PatchedFileRootValue{}, fmt.Errorf("%#v does not resolve to a file", symlinkPath)
			}
			d := r.stack.Peek()
			if err := d.setSymlink(loadOptions, *r.TerminalName, path.UNIXFormat.NewParser("TODO_SYMLINK_TARGET")); err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("failed to create symlink at %#v: %w", symlinkPath, err)
			}

			return createFileRootFromChangeTrackingDirectory(
				ctx,
				e,
				directoryCreationParameters,
				&rootDirectory,
			)
		default:
			return PatchedFileRootValue{}, errors.New("unknown output source type")
		}
	}

	// File refers to a source file. Extract the source file from
	// the correct repo. If the source file is a symbolic link, keep
	// on following them until we reach a file. Create a directory
	// hierarchy that contains the resulting file and all of the
	// symbolic links that we encountered along the way.
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	if !gotDirectoryCreationParameters || !gotDirectoryReaders {
		return PatchedFileRootValue{}, evaluation.ErrMissingDependency
	}
	resolver := reposFilePropertiesResolver[TReference, TMetadata]{
		context:          ctx,
		directoryReaders: directoryReaders,
		environment:      e,
	}

	var externalDirectory changeTrackingDirectory[TReference, TMetadata]
	symlinkRecordingComponentWalker := symlinkRecordingComponentWalker[TReference, TMetadata]{
		base:  &resolver,
		stack: util.NewNonEmptyStack(&externalDirectory),
	}

	if err := path.Resolve(
		path.UNIXFormat.NewParser(fileLabel.GetExternalRelativePath()),
		path.NewLoopDetectingScopeWalker(
			path.NewRelativeScopeWalker(&symlinkRecordingComponentWalker),
		),
	); err != nil {
		return PatchedFileRootValue{}, fmt.Errorf("failed to resolve path: %w", err)
	}

	resolvedFileProperties, err := resolver.getCurrentFileProperties()
	if err != nil {
		return PatchedFileRootValue{}, err
	}
	resolvedFile, err := newChangeTrackingFileFromFileProperties[TReference, TMetadata](resolvedFileProperties)
	if err != nil {
		return PatchedFileRootValue{}, err
	}

	// Resolving the file created all intermediate symbolic links.
	// Copy over the file they point to as well.
	if err := symlinkRecordingComponentWalker.stack.Peek().setFile(
		nil,
		*symlinkRecordingComponentWalker.terminalName,
		resolvedFile,
	); err != nil {
		return PatchedFileRootValue{}, err
	}

	return createFileRootFromChangeTrackingDirectory(
		ctx,
		e,
		directoryCreationParameters,
		&changeTrackingDirectory[TReference, TMetadata]{
			directories: map[path.Component]*changeTrackingDirectory[TReference, TMetadata]{
				model_starlark.ComponentExternal: &externalDirectory,
			},
		},
	)
}

func createFileRootFromChangeTrackingDirectory[TReference object.BasicReference, TMetadata model_core.WalkableReferenceMetadata](
	ctx context.Context,
	e FileRootEnvironment[TReference, TMetadata],
	directoryCreationParameters *model_filesystem.DirectoryCreationParameters,
	rootDirectory *changeTrackingDirectory[TReference, TMetadata],
) (PatchedFileRootValue, error) {
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
					objectCapturer: e,
				},
				directory: rootDirectory,
			},
			model_filesystem.NewSimpleDirectoryMerkleTreeCapturer[TMetadata](e),
			&createdRootDirectory,
		)
	})
	if err := group.Wait(); err != nil {
		return PatchedFileRootValue{}, err
	}

	return model_core.NewPatchedMessage(
		&model_analysis_pb.FileRoot_Value{
			RootDirectory: createdRootDirectory.Message.Message,
		},
		model_core.MapReferenceMetadataToWalkers(createdRootDirectory.Message.Patcher),
	), nil
}

type pathPrependingDirectory[TDirectory, TFile model_core.ReferenceMetadata] struct {
	components []path.Component
	directory  model_core.PatchedMessage[*model_filesystem_pb.DirectoryContents, TDirectory]
}

func (pathPrependingDirectory[TDirectory, TFile]) Close() error {
	return nil
}

func (d *pathPrependingDirectory[TDirectory, TFile]) ReadDir() ([]filesystem.FileInfo, error) {
	return []filesystem.FileInfo{
		filesystem.NewFileInfo(d.components[0], filesystem.FileTypeDirectory, false),
	}, nil
}

func (pathPrependingDirectory[TDirectory, TFile]) Readlink(name path.Component) (path.Parser, error) {
	panic("path prepending directory never contains symlinks")
}

func (d *pathPrependingDirectory[TDirectory, TFile]) EnterCapturableDirectory(name path.Component) (*model_filesystem.CreatedDirectory[TDirectory], model_filesystem.CapturableDirectory[TDirectory, TFile], error) {
	if len(d.components) > 1 {
		return nil, &pathPrependingDirectory[TDirectory, TFile]{
			components: d.components[1:],
			directory:  d.directory,
		}, nil
	}
	createdDirectory, err := model_filesystem.NewCreatedDirectoryBare(d.directory)
	return createdDirectory, nil, err
}

func (pathPrependingDirectory[TDirectory, TFile]) OpenForFileMerkleTreeCreation(name path.Component) (model_filesystem.CapturableFile[TFile], error) {
	panic("path prepending directory never contains regular files")
}

// symlinkRecordingComponentWalker is a decorator for
// path.ComponentWalker that monitors the paths that are being
// traversed, and copies over any symbolic links that are encountered
// into another directory hierarcy.
//
// This implementation is used when FileRoot is called against a source
// file. Any symbolic links that are encountered should be followed, but
// also be captured so that they appear in input roots of actions.
type symlinkRecordingComponentWalker[TReference object.BasicReference, TMetadata model_core.ReferenceMetadata] struct {
	base         path.ComponentWalker
	stack        util.NonEmptyStack[*changeTrackingDirectory[TReference, TMetadata]]
	terminalName *path.Component
}

func (cw *symlinkRecordingComponentWalker[TReference, TMetadata]) gotSymlink(name path.Component, r path.GotSymlink) (path.GotSymlink, error) {
	newBase, err := r.Parent.OnRelative()
	if err != nil {
		return path.GotSymlink{}, err
	}

	d := cw.stack.Peek()
	if err := d.setSymlink(nil, name, r.Target); err != nil {
		return path.GotSymlink{}, err
	}

	cw.base = newBase
	return path.GotSymlink{
		Parent: path.NewRelativeScopeWalker(cw),
		Target: r.Target,
	}, nil
}

func (cw *symlinkRecordingComponentWalker[TReference, TMetadata]) OnDirectory(name path.Component) (path.GotDirectoryOrSymlink, error) {
	result, err := cw.base.OnDirectory(name)
	if err != nil {
		return nil, err
	}
	switch r := result.(type) {
	case path.GotDirectory:
		if !r.IsReversible {
			return nil, errors.New("directory is not reversible, which this implementation assumes")
		}

		d := cw.stack.Peek()
		child, err := d.getOrCreateDirectory(name)
		if err != nil {
			return nil, err
		}
		cw.stack.Push(child)
		cw.base = r.Child

		return path.GotDirectory{
			Child:        cw,
			IsReversible: true,
		}, nil
	case path.GotSymlink:
		return cw.gotSymlink(name, r)
	default:
		panic("unexpected result type")
	}
}

func (cw *symlinkRecordingComponentWalker[TReference, TMetadata]) OnTerminal(name path.Component) (*path.GotSymlink, error) {
	result, err := cw.base.OnTerminal(name)
	if err != nil || result == nil {
		cw.terminalName = &name
		return result, err
	}
	newResult, err := cw.gotSymlink(name, *result)
	if err != nil {
		return nil, err
	}
	return &newResult, nil
}

func (cw *symlinkRecordingComponentWalker[TReference, TMetadata]) OnUp() (path.ComponentWalker, error) {
	parent, err := cw.base.OnUp()
	if err != nil {
		return nil, err
	}
	if _, ok := cw.stack.PopSingle(); !ok {
		return nil, errors.New("traversal escapes root directory")
	}
	cw.base = parent
	return cw, nil
}
