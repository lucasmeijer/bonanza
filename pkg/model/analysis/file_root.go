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
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
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
				File:            patchedFile.Message,
				DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
			},
			model_core.MapReferenceMetadataToWalkers(patchedFile.Patcher),
		),
	)
	if !gotDirectoryReaders || !targetOutput.IsSet() {
		return model_core.Message[*model_filesystem_pb.FileProperties, TReference]{}, evaluation.ErrMissingDependency
	}

	filePath, err := model_starlark.FileGetInputRootPath(f, nil)
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
		model_core.Message[*model_filesystem_pb.Directory, TReference]{},
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

func getPackageOutputDirectoryComponents[TReference object.BasicReference](configurationReference model_core.Message[*model_core_pb.DecodableReference, TReference], canonicalPackage label.CanonicalPackage, directoryLayout model_analysis_pb.DirectoryLayout) ([]path.Component, error) {
	var components []path.Component
	switch directoryLayout {
	case model_analysis_pb.DirectoryLayout_INPUT_ROOT:
		// TODO: Add more utility functions to pkg/label, so that we
		// don't need to call path.MustNewComponent() from here.
		configurationComponent, err := model_starlark.ConfigurationReferenceToComponent(configurationReference)
		if err != nil {
			return nil, err
		}
		components = append(
			components,
			model_starlark.ComponentBazelOut,
			path.MustNewComponent(configurationComponent),
			model_starlark.ComponentBin,
			model_starlark.ComponentExternal,
		)
	case model_analysis_pb.DirectoryLayout_RUNFILES:
	default:
		return nil, errors.New("unknown directory layout")
	}
	components = append(components, path.MustNewComponent(canonicalPackage.GetCanonicalRepo().String()))
	for packageComponent := range strings.FieldsFuncSeq(canonicalPackage.GetPackagePath(), func(r rune) bool { return r == '/' }) {
		components = append(components, path.MustNewComponent(packageComponent))
	}
	return components, nil
}

func fileGetPathInDirectoryLayout[TReference object.BasicReference](f model_core.Message[*model_starlark_pb.File, TReference], directoryLayout model_analysis_pb.DirectoryLayout) (string, error) {
	switch directoryLayout {
	case model_analysis_pb.DirectoryLayout_INPUT_ROOT:
		return model_starlark.FileGetInputRootPath(f, nil)
	case model_analysis_pb.DirectoryLayout_RUNFILES:
		return model_starlark.FileGetRunfilesPath(f)
	default:
		return "", errors.New("unknown directory layout")
	}
}

type changeTrackingDirectorySymlinkFollowingResolver[TReference object.BasicReference, TMetadata model_core.ReferenceMetadata] struct {
	loadOptions *changeTrackingDirectoryLoadOptions[TReference]

	stack util.NonEmptyStack[*changeTrackingDirectory[TReference, TMetadata]]
	file  *changeTrackingFile[TReference, TMetadata]
}

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnAbsolute() (path.ComponentWalker, error) {
	r.stack.PopAll()
	return r, nil
}

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnRelative() (path.ComponentWalker, error) {
	return r, nil
}

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnDriveLetter(driveLetter rune) (path.ComponentWalker, error) {
	return nil, errors.New("drive letters are not supported")
}

var errChangeTrackingDirectorySymlinkFollowingResolverFileNotFound = errors.New("file not found")

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnDirectory(name path.Component) (path.GotDirectoryOrSymlink, error) {
	d := r.stack.Peek()
	if err := d.maybeLoadContents(r.loadOptions); err != nil {
		return nil, err
	}

	if dChild, ok := d.directories[name]; ok {
		r.stack.Push(dChild)
		return path.GotDirectory{
			Child:        r,
			IsReversible: true,
		}, nil
	}
	if _, ok := d.files[name]; ok {
		return nil, errors.New("path resolves to a file, while a directory was expected")
	}
	if target, ok := d.symlinks[name]; ok {
		return path.GotSymlink{
			Parent: path.NewRelativeScopeWalker(r),
			Target: target,
		}, nil
	}
	return nil, errChangeTrackingDirectorySymlinkFollowingResolverFileNotFound
}

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnTerminal(name path.Component) (*path.GotSymlink, error) {
	d := r.stack.Peek()
	if err := d.maybeLoadContents(r.loadOptions); err != nil {
		return nil, err
	}

	if dChild, ok := d.directories[name]; ok {
		r.stack.Push(dChild)
		return nil, nil
	}
	if f, ok := d.files[name]; ok {
		r.file = f
		return nil, nil
	}
	if target, ok := d.symlinks[name]; ok {
		return &path.GotSymlink{
			Parent: path.NewRelativeScopeWalker(r),
			Target: target,
		}, nil
	}
	return nil, errChangeTrackingDirectorySymlinkFollowingResolverFileNotFound
}

func (r *changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]) OnUp() (path.ComponentWalker, error) {
	if _, ok := r.stack.PopSingle(); !ok {
		return nil, errors.New("path resolves to a location above the root directory")
	}
	return r, nil
}

// targetActionResultCopier is responsible for copying files out of
// the output root directories of target action results to a new
// directory hierarchy. This is used to extract individual outputs out
// of target action results (e.g., just "foo.o", even though the action
// also yields a "foo.d").
type targetActionResultCopier[TReference object.BasicReference, TMetadata model_core.ReferenceMetadata] struct {
	loadOptions        *changeTrackingDirectoryLoadOptions[TReference]
	directoriesScanned map[*changeTrackingDirectory[TReference, TMetadata]]struct{}
}

// copyFileAndDependencies extracts a single file or directory from the
// output root directory and places it in a new directory hierarchy.
func (c *targetActionResultCopier[TReference, TMetadata]) copyFileAndDependencies(
	originalOutputDirectory *changeTrackingDirectory[TReference, TMetadata],
	filePath string,
	fileType model_starlark_pb.File_Owner_Type,
) (*changeTrackingDirectory[TReference, TMetadata], error) {
	resolver := changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]{
		loadOptions: c.loadOptions,
		stack:       util.NewNonEmptyStack(originalOutputDirectory),
	}
	var trimmedOutputRootDirectory changeTrackingDirectory[TReference, TMetadata]
	symlinkRecordingComponentWalker := symlinkRecordingComponentWalker[TReference, TMetadata]{
		base:  &resolver,
		stack: util.NewNonEmptyStack(&trimmedOutputRootDirectory),
	}
	if err := path.Resolve(
		path.UNIXFormat.NewParser(filePath),
		path.NewLoopDetectingScopeWalker(
			path.NewRelativeScopeWalker(&symlinkRecordingComponentWalker),
		),
	); err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Resolving the file created all intermediate symbolic links.
	// Copy over the regular file or directory they point to as
	// well.
	switch fileType {
	case model_starlark_pb.File_Owner_FILE:
		if resolver.file == nil {
			return nil, errors.New("target action output is a directory, while a file was expected")
		}
		if err := symlinkRecordingComponentWalker.stack.Peek().setFile(
			c.loadOptions,
			*symlinkRecordingComponentWalker.terminalName,
			resolver.file,
		); err != nil {
			return nil, err
		}
	case model_starlark_pb.File_Owner_DIRECTORY:
		if resolver.file != nil {
			return nil, errors.New("target action output is a file, while a directory was expected")
		}
		if err := c.copyDirectoryAndDependencies(
			resolver.stack,
			&symlinkRecordingComponentWalker,
		); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown file type")
	}
	return &trimmedOutputRootDirectory, nil
}

// copyDirectoryAndDependencies is invoked by copyFileAndDependencies()
// to copy a directory from the target action output root into the new
// directory hierarchy. It also traverses the resulting directory to
// copy any files referenced via symbolic links contained within the
// directory.
func (c *targetActionResultCopier[TReference, TMetadata]) copyDirectoryAndDependencies(
	originalOutputDirectories util.NonEmptyStack[*changeTrackingDirectory[TReference, TMetadata]],
	trimmedOutputComponentWalker *symlinkRecordingComponentWalker[TReference, TMetadata],
) error {
	trimmedOutputDirectories := trimmedOutputComponentWalker.stack
	var newDirectory *changeTrackingDirectory[TReference, TMetadata]
	if terminalName := trimmedOutputComponentWalker.terminalName; terminalName == nil {
		// Symbolic link pointing to this directory contained a
		// trailing slash, meaning we're already within the
		// right directory.
		newDirectory = trimmedOutputDirectories.Peek()
	} else {
		// Symbolic link pointing to this directory did not
		// contain a trailing slash. This means the target
		// directory did not get created for us.
		var err error
		newDirectory, err = trimmedOutputComponentWalker.stack.Peek().getOrCreateDirectory(*trimmedOutputComponentWalker.terminalName)
		if err != nil {
			return err
		}
		trimmedOutputDirectories = trimmedOutputDirectories.Copy()
		trimmedOutputDirectories.Push(newDirectory)
	}
	if err := newDirectory.mergeDirectory(originalOutputDirectories.Peek(), c.loadOptions); err != nil {
		return err
	}
	return c.scanDirectoryDependencies(originalOutputDirectories, trimmedOutputDirectories, 0)
}

// scanDirectoryDependencies visits all symbolic links in a directory
// hierarchy and ensures their targets are copied over into the newly
// created directory hierarchy as well.
func (c *targetActionResultCopier[TReference, TMetadata]) scanDirectoryDependencies(
	originalOutputDirectories util.NonEmptyStack[*changeTrackingDirectory[TReference, TMetadata]],
	trimmedOutputDirectories util.NonEmptyStack[*changeTrackingDirectory[TReference, TMetadata]],
	maximumEscapementLevels uint32,
) error {
	// Prevent cyclic scanning of directories.
	trimmedOutputDirectory := trimmedOutputDirectories.Peek()
	if _, ok := c.directoriesScanned[trimmedOutputDirectory]; ok {
		return nil
	}
	c.directoriesScanned[trimmedOutputDirectory] = struct{}{}

	if trimmedOutputDirectory.maximumSymlinkEscapementLevelsAtMost(maximumEscapementLevels) {
		// This directory does not contain any symlinks that
		// escape the directory that was copied. This means that
		// there is no need to traverse it.
		return nil
	}

	originalOutputDirectory := originalOutputDirectories.Peek()
	if err := originalOutputDirectory.maybeLoadContents(c.loadOptions); err != nil {
		return err
	}
	if err := trimmedOutputDirectory.maybeLoadContents(c.loadOptions); err != nil {
		return err
	}

	for name, trimmedOutputChildDirectory := range trimmedOutputDirectory.directories {
		originalOutputDirectories.Push(originalOutputDirectory.directories[name])
		trimmedOutputDirectories.Push(trimmedOutputChildDirectory)
		if err := c.scanDirectoryDependencies(
			originalOutputDirectories,
			trimmedOutputDirectories,
			maximumEscapementLevels+1,
		); err != nil {
			return err
		}
		if _, ok := originalOutputDirectories.PopSingle(); !ok {
			panic("bad directory stack handling")
		}
		if _, ok := trimmedOutputDirectories.PopSingle(); !ok {
			panic("bad directory stack handling")
		}
	}

	for name, target := range trimmedOutputDirectory.symlinks {
		resolver := changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]{
			loadOptions: c.loadOptions,
			stack:       originalOutputDirectories.Copy(),
		}
		symlinkRecordingComponentWalker := symlinkRecordingComponentWalker[TReference, TMetadata]{
			base:  &resolver,
			stack: trimmedOutputDirectories.Copy(),
		}
		if err := path.Resolve(
			target,
			path.NewLoopDetectingScopeWalker(
				path.NewRelativeScopeWalker(&symlinkRecordingComponentWalker),
			),
		); err != nil {
			return fmt.Errorf("failed to resolve symlink %#v: %w", name.String(), err)
		}

		if resolver.file != nil {
			if err := symlinkRecordingComponentWalker.stack.Peek().setFile(
				c.loadOptions,
				*symlinkRecordingComponentWalker.terminalName,
				resolver.file,
			); err != nil {
				return err
			}
		} else {
			if err := c.copyDirectoryAndDependencies(
				resolver.stack,
				&symlinkRecordingComponentWalker,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeFileRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.FileRoot_Key, TReference], e FileRootEnvironment[TReference, TMetadata]) (PatchedFileRootValue, error) {
	f := model_core.Nested(key, key.Message.File)
	if f.Message == nil {
		return PatchedFileRootValue{}, fmt.Errorf("no file provided")
	}
	fileLabel, err := label.NewCanonicalLabel(f.Message.Label)
	if err != nil {
		return PatchedFileRootValue{}, fmt.Errorf("invalid file label: %w", err)
	}

	if o := f.Message.Owner; o != nil {
		targetName, err := label.NewTargetName(o.TargetName)
		if err != nil {
			return PatchedFileRootValue{}, fmt.Errorf("invalid target name: %w", err)
		}

		configurationReference := model_core.Nested(f, o.ConfigurationReference)
		targetLabel := fileLabel.GetCanonicalPackage().AppendTargetName(targetName)
		patchedConfigurationReference := model_core.Patch(e, configurationReference)
		output := e.GetTargetOutputValue(
			model_core.NewPatchedMessage(
				&model_analysis_pb.TargetOutput_Key{
					Label:                  targetLabel.String(),
					ConfigurationReference: patchedConfigurationReference.Message,
					PackageRelativePath:    fileLabel.GetTargetName().String(),
				},
				model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
			),
		)
		if !output.IsSet() {
			return PatchedFileRootValue{}, evaluation.ErrMissingDependency
		}

		switch source := output.Message.Definition.GetSource().(type) {
		case *model_analysis_pb.TargetOutputDefinition_ActionId:
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
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
			if !gotDirectoryCreationParameters || !gotDirectoryReaders || !targetActionResult.IsSet() {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			// Target actions may emit multiple outputs, but
			// we should return a file root that only
			// contains a single output. Create a new
			// directory hierarchy that only contains a copy
			// of the file that was requested (and any
			// symlink targets).
			originalOutputRootDirectory := changeTrackingDirectory[TReference, TMetadata]{
				unmodifiedDirectory: model_core.Nested(
					targetActionResult,
					&model_filesystem_pb.Directory{
						Contents: &model_filesystem_pb.Directory_ContentsInline{
							ContentsInline: targetActionResult.Message.OutputRoot,
						},
					},
				),
			}
			filePath, err := model_starlark.FileGetInputRootPath(f, nil)
			if err != nil {
				return PatchedFileRootValue{}, err
			}
			loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
				context:                 ctx,
				directoryContentsReader: directoryReaders.DirectoryContents,
				leavesReader:            directoryReaders.Leaves,
			}
			copier := targetActionResultCopier[TReference, TMetadata]{
				loadOptions:        loadOptions,
				directoriesScanned: map[*changeTrackingDirectory[TReference, TMetadata]]struct{}{},
			}
			trimmedOutputRootDirectory, err := copier.copyFileAndDependencies(&originalOutputRootDirectory, filePath, o.Type)
			if err != nil {
				if !errors.Is(err, errChangeTrackingDirectorySymlinkFollowingResolverFileNotFound) {
					return PatchedFileRootValue{}, err
				}

				// Target action outputs contain one or
				// more symbolic links pointing to files
				// that were provided as inputs. Merge
				// the input root of the action with the
				// outputs and retry.
				//
				// We only try to do this when needed,
				// as it's only rarely the case that
				// such dependencies exist.
				patchedConfigurationReference := model_core.Patch(e, configurationReference)
				targetActionInputRoot := e.GetTargetActionInputRootValue(
					model_core.NewPatchedMessage(
						&model_analysis_pb.TargetActionInputRoot_Key{
							Id: &model_analysis_pb.TargetActionId{
								Label:                  targetLabel.String(),
								ConfigurationReference: patchedConfigurationReference.Message,
								ActionId:               source.ActionId,
							},
						},
						model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
					),
				)
				if !targetActionInputRoot.IsSet() {
					return PatchedFileRootValue{}, evaluation.ErrMissingDependency
				}
				if err := originalOutputRootDirectory.mergeDirectoryMessage(
					model_core.Nested(targetActionInputRoot, &model_filesystem_pb.Directory{
						Contents: &model_filesystem_pb.Directory_ContentsExternal{
							ContentsExternal: targetActionInputRoot.Message.InputRootReference,
						},
					}),
					loadOptions,
				); err != nil {
					return PatchedFileRootValue{}, err
				}

				copier := targetActionResultCopier[TReference, TMetadata]{
					loadOptions:        loadOptions,
					directoriesScanned: map[*changeTrackingDirectory[TReference, TMetadata]]struct{}{},
				}
				trimmedOutputRootDirectory, err = copier.copyFileAndDependencies(&originalOutputRootDirectory, filePath, o.Type)
				if err != nil {
					return PatchedFileRootValue{}, err
				}
			}

			switch key.Message.DirectoryLayout {
			case model_analysis_pb.DirectoryLayout_INPUT_ROOT:
				return createFileRootFromChangeTrackingDirectory(
					ctx,
					e,
					directoryReaders.DirectoryContents,
					directoryCreationParameters,
					trimmedOutputRootDirectory,
				)
			case model_analysis_pb.DirectoryLayout_RUNFILES:
				// Merge the bazel-out/*/bin/external/*
				// directories into external/*.
				externalDirectory, err := trimmedOutputRootDirectory.getOrCreateDirectory(model_starlark.ComponentExternal)
				if err != nil {
					return PatchedFileRootValue{}, err
				}
				if bazelOutDirectory, ok := trimmedOutputRootDirectory.directories[model_starlark.ComponentBazelOut]; ok {
					for configurationName, configurationDirectory := range bazelOutDirectory.directories {
						if binDirectory, ok := configurationDirectory.directories[model_starlark.ComponentBin]; ok {
							if configurationExternalDirectory, ok := binDirectory.directories[model_starlark.ComponentExternal]; ok {
								if err := externalDirectory.mergeDirectory(configurationExternalDirectory, loadOptions); err != nil {
									return PatchedFileRootValue{}, fmt.Errorf("failed to merge outputs of configuration %#v: %w", configurationName.String())
								}
							}
						}
					}
				}

				// TODO: We still need to repair
				// symbolic links that point to source
				// files or files belonging to different
				// configurations.

				return createFileRootFromChangeTrackingDirectory(
					ctx,
					e,
					directoryReaders.DirectoryContents,
					directoryCreationParameters,
					externalDirectory,
				)
			default:
				return PatchedFileRootValue{}, errors.New("unknown directory layout")
			}

		case *model_analysis_pb.TargetOutputDefinition_ExpandTemplate_:
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

			components, err := getPackageOutputDirectoryComponents(configurationReference, fileLabel.GetCanonicalPackage(), key.Message.DirectoryLayout)
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
		case *model_analysis_pb.TargetOutputDefinition_StaticPackageDirectory:
			// Output file was already computed during configuration.
			// For example by calling ctx.actions.write() or
			// ctx.actions.symlink(target_path=...).
			//
			// Wrap the package directory to make it an input root.
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			if !gotDirectoryCreationParameters {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			components, err := getPackageOutputDirectoryComponents(configurationReference, fileLabel.GetCanonicalPackage(), key.Message.DirectoryLayout)
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
		case *model_analysis_pb.TargetOutputDefinition_Symlink_:
			// Symlink to another file. Obtain the root of
			// the target and add a symlink to it.
			directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
			directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
			symlinkTargetFile := model_core.Nested(output, source.Symlink.Target)
			patchedSymlinkTargetFile := model_core.Patch(e, symlinkTargetFile)
			symlinkTarget := e.GetFileRootValue(
				model_core.NewPatchedMessage(
					&model_analysis_pb.FileRoot_Key{
						File:            patchedSymlinkTargetFile.Message,
						DirectoryLayout: key.Message.DirectoryLayout,
					},
					model_core.MapReferenceMetadataToWalkers(patchedSymlinkTargetFile.Patcher),
				),
			)
			if !gotDirectoryCreationParameters || !gotDirectoryReaders || !symlinkTarget.IsSet() {
				return PatchedFileRootValue{}, evaluation.ErrMissingDependency
			}

			symlinkPath, err := fileGetPathInDirectoryLayout(f, key.Message.DirectoryLayout)
			if err != nil {
				return PatchedFileRootValue{}, err
			}

			rootDirectory := changeTrackingDirectory[TReference, TMetadata]{
				unmodifiedDirectory: model_core.Nested(symlinkTarget, &model_filesystem_pb.Directory{
					Contents: &model_filesystem_pb.Directory_ContentsInline{
						ContentsInline: symlinkTarget.Message.RootDirectory,
					},
				}),
			}
			loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
				context:                 ctx,
				directoryContentsReader: directoryReaders.DirectoryContents,
				leavesReader:            directoryReaders.Leaves,
			}

			// Validate the type and properties of the target file.
			targetPath, err := fileGetPathInDirectoryLayout(symlinkTargetFile, key.Message.DirectoryLayout)
			if err != nil {
				return PatchedFileRootValue{}, err
			}
			targetResolver := changeTrackingDirectorySymlinkFollowingResolver[TReference, TMetadata]{
				loadOptions: loadOptions,
				stack:       util.NewNonEmptyStack(&rootDirectory),
			}
			if err := path.Resolve(
				path.UNIXFormat.NewParser(targetPath),
				path.NewLoopDetectingScopeWalker(
					path.NewRelativeScopeWalker(&targetResolver),
				),
			); err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("cannot resolve %#v: %w", targetPath, err)
			}
			targetFile := targetResolver.file
			switch o.Type {
			case model_starlark_pb.File_Owner_FILE:
				if targetFile == nil {
					return PatchedFileRootValue{}, fmt.Errorf("path %#v resolves to a directory, while a file was expected", targetPath)
				}
				if source.Symlink.IsExecutable && !targetFile.isExecutable {
					return PatchedFileRootValue{}, fmt.Errorf("file at path %#v is not executable, even though it should be", targetPath)
				}
			case model_starlark_pb.File_Owner_DIRECTORY:
				if targetFile != nil {
					return PatchedFileRootValue{}, fmt.Errorf("path %#v resolves to a file, while a directory was expected", targetPath)
				}
			default:
				return PatchedFileRootValue{}, errors.New("unknown file type")
			}

			symlinkResolver := changeTrackingDirectoryNewFileResolver[TReference, TMetadata]{
				loadOptions: loadOptions,
				stack:       util.NewNonEmptyStack(&rootDirectory),
			}
			if err := path.Resolve(path.UNIXFormat.NewParser(symlinkPath), &symlinkResolver); err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("cannot resolve %#v: %w", symlinkPath, err)
			}
			if symlinkResolver.TerminalName == nil {
				return PatchedFileRootValue{}, fmt.Errorf("%#v does not resolve to a file", symlinkPath)
			}

			// Make the target of the symlink relative to
			// the directory in which it is contained.
			// Remove leading components of the target that
			// are equal to those of the symlink's path.
			equalComponentsBytes := 0
			for i := 0; i < len(symlinkPath) && i < len(targetPath) && symlinkPath[i] == targetPath[i]; i++ {
				if symlinkPath[i] == '/' {
					equalComponentsBytes = i + 1
				}
			}
			relativeTargetPath := strings.Repeat("../", strings.Count(symlinkPath[equalComponentsBytes:], "/")) + targetPath[equalComponentsBytes:]

			d := symlinkResolver.stack.Peek()
			if err := d.setSymlink(loadOptions, *symlinkResolver.TerminalName, path.UNIXFormat.NewParser(relativeTargetPath)); err != nil {
				return PatchedFileRootValue{}, fmt.Errorf("failed to create symlink at %#v: %w", symlinkPath, err)
			}

			return createFileRootFromChangeTrackingDirectory(
				ctx,
				e,
				directoryReaders.DirectoryContents,
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

	// Prepend "external" depending on whether this needs to go into
	// the input root or the runfiles directory.
	var rootDirectory *changeTrackingDirectory[TReference, TMetadata]
	switch key.Message.DirectoryLayout {
	case model_analysis_pb.DirectoryLayout_INPUT_ROOT:
		rootDirectory = &changeTrackingDirectory[TReference, TMetadata]{
			directories: map[path.Component]*changeTrackingDirectory[TReference, TMetadata]{
				model_starlark.ComponentExternal: &externalDirectory,
			},
		}
	case model_analysis_pb.DirectoryLayout_RUNFILES:
		rootDirectory = &externalDirectory
	default:
		return PatchedFileRootValue{}, errors.New("unknown directory layout")
	}
	return createFileRootFromChangeTrackingDirectory(
		ctx,
		e,
		directoryReaders.DirectoryContents,
		directoryCreationParameters,
		rootDirectory,
	)
}

func createFileRootFromChangeTrackingDirectory[TReference object.BasicReference, TMetadata model_core.WalkableReferenceMetadata](
	ctx context.Context,
	e FileRootEnvironment[TReference, TMetadata],
	directoryContentsReader model_parser.ParsedObjectReader[model_core.Decodable[TReference], model_core.Message[*model_filesystem_pb.DirectoryContents, TReference]],
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
					context:                 ctx,
					directoryContentsReader: directoryContentsReader,
					objectCapturer:          e,
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

type FileRootEnvironmentForTesting FileRootEnvironment[model_core.CreatedObjectTree, model_core.CreatedObjectTree]
