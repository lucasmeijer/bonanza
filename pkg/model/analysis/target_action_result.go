package analysis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/starlark/unpack"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.starlark.net/starlark"
)

// splitArgsTemplate preprocesses format strings that are provided to
// Args.add*()'s format, format_each, and format_joined. These format
// strings are only permitted to contain zero or more "%%" directives,
// and exactly one "%s" directive. This means we can precompute strings
// to prepend and append to the observed values.
func splitArgsTemplate(template string) (string, string, error) {
	gotPercent := false
	gotPlaceholder := false
	var prefix strings.Builder
	var suffix strings.Builder
	for _, r := range template {
		if gotPercent {
			switch r {
			case '%':
				if gotPlaceholder {
					suffix.WriteByte('%')
				} else {
					prefix.WriteByte('%')
				}
			case 's':
				if gotPlaceholder {
					return "", "", errors.New("template contains multiple %s substitution placeholders")
				}
				gotPlaceholder = true
			default:
				return "", "", fmt.Errorf("unsupported conversion specifier %q", r)
			}
			gotPercent = false
		} else {
			switch r {
			case '%':
				gotPercent = true
			default:
				if gotPlaceholder {
					suffix.WriteRune(r)
				} else {
					prefix.WriteRune(r)
				}
			}
		}
	}
	if gotPercent {
		return "", "", errors.New("unterminated % directive")
	}
	if !gotPlaceholder {
		return "", "", errors.New("template contains no %s substitution placeholders")
	}
	return prefix.String(), suffix.String(), nil
}

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionResult_Key, TReference], e TargetActionResultEnvironment[TReference, TMetadata]) (PatchedTargetActionResultValue, error) {
	commandEncoder, gotCommandEncoder := e.GetCommandEncoderObjectValue(&model_analysis_pb.CommandEncoderObject_Key{})
	allBuiltinsModulesNames := e.GetBuiltinsModuleNamesValue(&model_analysis_pb.BuiltinsModuleNames_Key{})
	patchedConfigurationReference := model_core.Patch(e, model_core.Nested(key, key.Message.ConfigurationReference))
	configuredTarget := e.GetConfiguredTargetValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ConfiguredTarget_Key{
				Label:                  key.Message.Label,
				ConfigurationReference: patchedConfigurationReference.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
		),
	)
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryCreationParametersMessage := e.GetDirectoryCreationParametersValue(&model_analysis_pb.DirectoryCreationParameters_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	fileCreationParametersMessage := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !allBuiltinsModulesNames.IsSet() ||
		!gotCommandEncoder ||
		!configuredTarget.IsSet() ||
		!gotDirectoryCreationParameters ||
		!gotDirectoryReaders ||
		!directoryCreationParametersMessage.IsSet() ||
		!fileCreationParametersMessage.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	// Look up the action within the configured target.
	// TODO: Should this be moved into a separate function, so that
	// changes to a single action does not require others to be
	// recomputed?
	actionID := key.Message.ActionId
	action, err := btree.Find(
		ctx,
		c.configuredTargetActionReader,
		model_core.Nested(configuredTarget, configuredTarget.Message.Actions),
		func(entry *model_analysis_pb.ConfiguredTarget_Value_Action) (int, *model_core_pb.Reference) {
			switch level := entry.Level.(type) {
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_:
				return bytes.Compare(actionID, level.Leaf.Id), nil
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Parent_:
				return bytes.Compare(actionID, level.Parent.FirstId), level.Parent.Reference
			default:
				return 0, nil
			}
		},
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}
	if !action.IsSet() {
		return PatchedTargetActionResultValue{}, errors.New("target does not yield an action with the provided identifier")
	}
	actionLevel, ok := action.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_)
	if !ok {
		return PatchedTargetActionResultValue{}, errors.New("action is not a leaf")
	}
	actionLeaf := actionLevel.Leaf

	// Construct the list of command line arguments.
	// TODO: Respect use_param_file().
	referenceFormat := c.getReferenceFormat()
	argumentsBuilder := newArgumentsBuilder(commandEncoder, referenceFormat, e)
	valueDecodingOptions := c.getValueDecodingOptions(ctx, func(resolvedLabel label.ResolvedLabel) (starlark.Value, error) {
		return model_starlark.NewLabel[TReference, TMetadata](resolvedLabel), nil
	})
	thread := c.newStarlarkThread(ctx, e, allBuiltinsModulesNames.Message.BuiltinsModuleNames)
	var errIterArgs error
	for args := range btree.AllLeaves(
		ctx,
		c.argsReader,
		model_core.Nested(action, actionLeaf.Arguments),
		/* traverser = */ func(element model_core.Message[*model_analysis_pb.Args, TReference]) (*model_core_pb.Reference, error) {
			if level, ok := element.Message.Level.(*model_analysis_pb.Args_Parent_); ok {
				return level.Parent.Reference, nil
			}
			return nil, nil
		},
		&errIterArgs,
	) {
		argsLeaf, ok := args.Message.Level.(*model_analysis_pb.Args_Leaf_)
		if !ok {
			return PatchedTargetActionResultValue{}, errors.New("args entry is not a leaf")
		}
		var errIterAdd error
		for add := range btree.AllLeaves(
			ctx,
			c.argsAddReader,
			model_core.Nested(args, argsLeaf.Leaf.Adds),
			/* traverser = */ func(element model_core.Message[*model_analysis_pb.Args_Leaf_Add, TReference]) (*model_core_pb.Reference, error) {
				if level, ok := element.Message.Level.(*model_analysis_pb.Args_Leaf_Add_Parent_); ok {
					return level.Parent.Reference, nil
				}
				return nil, nil
			},
			&errIterAdd,
		) {
			addLeaf, ok := add.Message.Level.(*model_analysis_pb.Args_Leaf_Add_Leaf_)
			if !ok {
				return PatchedTargetActionResultValue{}, errors.New("args.add*() entry is not a leaf")
			}

			values, err := model_starlark.DecodeValue[TReference, TMetadata](
				model_core.Nested(add, addLeaf.Leaf.Values),
				/* currentIdentifier = */ nil,
				valueDecodingOptions,
			)
			if err != nil {
				return PatchedTargetActionResultValue{}, err
			}
			var valuesIter iter.Seq[starlark.Value]
			switch typedValues := values.(type) {
			case *model_starlark.Depset[TReference, TMetadata]:
				list, err := typedValues.ToList(thread)
				if err != nil {
					return PatchedTargetActionResultValue{}, err
				}
				valuesIter = slices.Values(list)
			case starlark.Iterable:
				valuesIter = starlark.Elements(typedValues)
			default:
				return PatchedTargetActionResultValue{}, errors.New("args.add*() value is not a depset or list")
			}

			// Apply the following transformation steps:
			// https://bazel.build/rules/lib/builtins/Args#add_all

			// Step 1: Each directory File item is replaced by all
			// Files recursively contained in that directory.
			if addLeaf.Leaf.ExpandDirectories {
				var expandedValues []starlark.Value
				for v := range valuesIter {
					if f, ok := v.(*model_starlark.File[TReference, TMetadata]); ok {
						if d := f.GetDefinition(); d.Message.Type == model_starlark_pb.File_DIRECTORY {
							return PatchedTargetActionResultValue{}, errors.New("TODO: Implement directory expansion!")
						}
					}
					expandedValues = append(expandedValues, v)
				}
				valuesIter = slices.Values(expandedValues)
			}

			// Step 2: If map_each is given, it is applied
			// to each item, and the resulting lists of
			// strings are concatenated to form the initial
			// argument list. Otherwise, the initial
			// argument list is the result of applying the
			// standard conversion to each item.
			var stringValues []string
			if mapEach := addLeaf.Leaf.MapEach; mapEach != nil {
				mapEachFunc := model_starlark.NewNamedFunction(
					model_starlark.NewProtoNamedFunctionDefinition[TReference, TMetadata](
						model_core.Nested(add, mapEach),
					),
				)
				for v := range valuesIter {
					returnValue, err := starlark.Call(
						thread,
						mapEachFunc,
						starlark.Tuple{
							v,
							// TODO: Provide a DirectoryExpander!
						},
						nil,
					)
					if err != nil {
						return PatchedTargetActionResultValue{}, err
					}
					var s []string
					if err := unpack.IfNotNone(unpack.Or([]unpack.UnpackerInto[[]string]{
						unpack.Singleton(unpack.String),
						unpack.List(unpack.String),
					})).UnpackInto(thread, returnValue, &s); err != nil {
						return PatchedTargetActionResultValue{}, fmt.Errorf("failed to unpack map function return value: %w", err)
					}
					stringValues = append(stringValues, s...)
				}
			} else {
				// No mapping function provided. Apply
				// standard conversion rules.
				for v := range valuesIter {
					var s string
					switch typedV := v.(type) {
					case starlark.String:
						s = string(typedV)
					case *model_starlark.File[TReference, TMetadata]:
						s, err = model_starlark.FileGetPath(typedV.GetDefinition())
						if err != nil {
							return PatchedTargetActionResultValue{}, err
						}
					case model_starlark.Label[TReference, TMetadata]:
						s = typedV.String()
					default:
						return PatchedTargetActionResultValue{}, fmt.Errorf("argument value is of type %#v, while a string, File or Label were expected", typedV.Type())
					}
					stringValues = append(stringValues, s)
				}
			}

			if len(stringValues) == 0 && addLeaf.Leaf.OmitIfEmpty {
				continue
			}

			// Step 6 (early): Except in the case that the
			// list is empty and omit_if_empty is true,
			// start_with is inserted as the first argument,
			// if it is given.
			if startWith := addLeaf.Leaf.StartWith; startWith != nil {
				if err := argumentsBuilder.PushChild(
					model_core.NewSimplePatchedMessage[TMetadata](
						&model_command_pb.ArgumentList_Element{
							Level: &model_command_pb.ArgumentList_Element_Leaf{
								Leaf: startWith.Value,
							},
						},
					),
				); err != nil {
					return PatchedTargetActionResultValue{}, err
				}
			}

			formatEachPrefix, formatEachSuffix, err := splitArgsTemplate(addLeaf.Leaf.FormatEach)
			if err != nil {
				return PatchedTargetActionResultValue{}, fmt.Errorf("invalid value for args.add_*(format_each=%#v): %w", addLeaf.Leaf.FormatEach, err)
			}
			var seen map[string]struct{}
			if addLeaf.Leaf.Uniquify {
				seen = make(map[string]struct{}, len(stringValues))
			}

			switch style := addLeaf.Leaf.Style.(type) {
			case *model_analysis_pb.Args_Leaf_Add_Leaf_Separate_:
				for _, v := range stringValues {
					// Step 4: If uniquify is true,
					// duplicate arguments are removed.
					// The first occurrence is the one
					// that remains.
					if seen != nil {
						if _, ok := seen[v]; ok {
							continue
						}
						seen[v] = struct{}{}
					}

					// Step 5: If a before_each string
					// is given, it is inserted as a new
					// argument before each existing
					// argument in the list.
					if beforeEach := style.Separate.BeforeEach; beforeEach != nil {
						if err := argumentsBuilder.PushChild(
							model_core.NewSimplePatchedMessage[TMetadata](
								&model_command_pb.ArgumentList_Element{
									Level: &model_command_pb.ArgumentList_Element_Leaf{
										Leaf: beforeEach.Value,
									},
								},
							),
						); err != nil {
							return PatchedTargetActionResultValue{}, err
						}
					}

					// Step 3: Each argument in the list
					// is formatted with format_each.
					if err := argumentsBuilder.PushChild(
						model_core.NewSimplePatchedMessage[TMetadata](
							&model_command_pb.ArgumentList_Element{
								Level: &model_command_pb.ArgumentList_Element_Leaf{
									Leaf: formatEachPrefix + v + formatEachSuffix,
								},
							},
						),
					); err != nil {
						return PatchedTargetActionResultValue{}, err
					}
				}

				// Step 6: Except in the case that the
				// list is empty and omit_if_empty is
				// true, terminate_with is inserted as
				// the last argument, if it is given.
				if terminateWith := style.Separate.TerminateWith; terminateWith != nil {
					if err := argumentsBuilder.PushChild(
						model_core.NewSimplePatchedMessage[TMetadata](
							&model_command_pb.ArgumentList_Element{
								Level: &model_command_pb.ArgumentList_Element_Leaf{
									Leaf: terminateWith.Value,
								},
							},
						),
					); err != nil {
						return PatchedTargetActionResultValue{}, err
					}
				}
			case *model_analysis_pb.Args_Leaf_Add_Leaf_Joined_:
				formatJoinedPrefix, formatJoinedSuffix, err := splitArgsTemplate(style.Joined.FormatJoined)
				if err != nil {
					return PatchedTargetActionResultValue{}, fmt.Errorf("invalid value for args.add_*(format_joined=%#v): %w", style.Joined.FormatJoined, err)
				}
				var joinedValues strings.Builder
				joinedValues.WriteString(formatJoinedPrefix)

				for i, v := range stringValues {
					// Step 4: If uniquify is true,
					// duplicate arguments are removed.
					// The first occurrence is the one
					// that remains.
					if seen != nil {
						if _, ok := seen[v]; ok {
							continue
						}
						seen[v] = struct{}{}
					}

					if i > 0 {
						joinedValues.WriteString(style.Joined.JoinWith)
					}

					// Step 3: Each argument in the list
					// is formatted with format_each.
					joinedValues.WriteString(formatEachPrefix)
					joinedValues.WriteString(v)
					joinedValues.WriteString(formatEachSuffix)
				}

				joinedValues.WriteString(formatJoinedSuffix)
				if err := argumentsBuilder.PushChild(
					model_core.NewSimplePatchedMessage[TMetadata](
						&model_command_pb.ArgumentList_Element{
							Level: &model_command_pb.ArgumentList_Element_Leaf{
								Leaf: joinedValues.String(),
							},
						},
					),
				); err != nil {
					return PatchedTargetActionResultValue{}, err
				}
			default:
				return PatchedTargetActionResultValue{}, errors.New("unknown args.add*() style")
			}
		}
		if errIterAdd != nil {
			return PatchedTargetActionResultValue{}, errIterAdd
		}
	}
	if errIterArgs != nil {
		return PatchedTargetActionResultValue{}, errIterArgs
	}
	argumentsList, err := argumentsBuilder.FinalizeList()
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	// Construct the command of the action.
	createdCommand, err := model_core.MarshalAndEncodePatchedMessage(
		model_core.NewPatchedMessage(
			&model_command_pb.Command{
				Arguments: argumentsList.Message,
				// EnvironmentVariables: enironmentVariables,
				DirectoryCreationParameters: directoryCreationParametersMessage.Message.DirectoryCreationParameters,
				FileCreationParameters:      fileCreationParametersMessage.Message.FileCreationParameters,
				// OutputPathPattern: outputPathPattern,
				WorkingDirectory: (*path.Trace)(nil).GetUNIXString(),
			},
			argumentsList.Patcher,
		),
		referenceFormat,
		commandEncoder,
	)

	// Construct the input root of the action.
	var rootDirectory changeTrackingDirectory[TReference, TMetadata]
	loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
		context:         ctx,
		directoryReader: directoryReaders.Directory,
		leavesReader:    directoryReaders.Leaves,
	}
	if err := addFilesToChangeTrackingDirectory(
		e,
		model_core.Nested(action, actionLeaf.Inputs),
		&rootDirectory,
		loadOptions,
	); err != nil {
		return PatchedTargetActionResultValue{}, err
	}
	// TODO: We need to add runfiles for the tools!
	if err := addFilesToChangeTrackingDirectory(
		e,
		model_core.Nested(action, actionLeaf.Tools),
		&rootDirectory,
		loadOptions,
	); err != nil {
		return PatchedTargetActionResultValue{}, err
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
					context:         ctx,
					directoryReader: directoryReaders.Directory,
					objectCapturer:  e,
				},
				directory: &rootDirectory,
			},
			model_filesystem.NewSimpleDirectoryMerkleTreeCapturer[TMetadata](e),
			&createdRootDirectory,
		)
	})
	if err := group.Wait(); err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	rootDirectoryObject, err := model_core.MarshalAndEncodePatchedMessage(
		createdRootDirectory.Message,
		referenceFormat,
		directoryCreationParameters.GetEncoder(),
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	actionResult := e.GetActionResultValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ActionResult_Key{
				CommandReference: patcher.AddReference(
					createdCommand.Contents.GetReference(),
					e.CaptureCreatedObject(createdCommand),
				),
				// TODO: Should we make the execution
				// timeout on build actions configurable?
				// Bazel with REv2 does not set this field
				// for build actions, relying on the cluster
				// to pick a default.
				ExecutionTimeout:   &durationpb.Duration{Seconds: 3600},
				ExitCodeMustBeZero: true,
				InputRootReference: patcher.AddReference(
					rootDirectoryObject.Contents.GetReference(),
					e.CaptureCreatedObject(rootDirectoryObject),
				),
				PlatformPkixPublicKey: actionLeaf.PlatformPkixPublicKey,
			},
			model_core.MapReferenceMetadataToWalkers(patcher),
		),
	)
	if !actionResult.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	return PatchedTargetActionResultValue{}, errors.New("TODO: Invoke action!")
}
