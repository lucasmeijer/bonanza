package analysis

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	"github.com/buildbarn/bonanza/pkg/label"
	model_command "github.com/buildbarn/bonanza/pkg/model/command"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	"github.com/buildbarn/bonanza/pkg/model/core/inlinedtree"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/starlark/unpack"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/proto"

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

type expandFileIfDirectoryEnvironment[TReference, TMetadata any] interface {
	model_core.ExistingObjectCapturer[TReference, TMetadata]

	GetFileRootValue(key model_core.PatchedMessage[*model_analysis_pb.FileRoot_Key, dag.ObjectContentsWalker]) model_core.Message[*model_analysis_pb.FileRoot_Value, TReference]
}

// expandFileIfDirectory checks whether a File provided to Args.add*()
// corresponds to a directory. If so, it expands it to a sequence of
// File objects corresponding to its children.
func (c *baseComputer[TReference, TMetadata]) expandFileIfDirectory(e expandFileIfDirectoryEnvironment[TReference, TMetadata], file *model_starlark.File[TReference, TMetadata], errOut *error) iter.Seq[*model_starlark.File[TReference, TMetadata]] {
	d := file.GetDefinition()
	if d.Message.Type != model_starlark_pb.File_DIRECTORY {
		return func(yield func(*model_starlark.File[TReference, TMetadata]) bool) {
			yield(file)
		}
	}

	patchedFile := model_core.Patch(e, d)
	fileRoot := e.GetFileRootValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.FileRoot_Key{
				File: patchedFile.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedFile.Patcher),
		),
	)
	if !fileRoot.IsSet() {
		*errOut = evaluation.ErrMissingDependency
		return func(yield func(*model_starlark.File[TReference, TMetadata]) bool) {}
	}

	*errOut = errors.New("TODO")
	return func(yield func(*model_starlark.File[TReference, TMetadata]) bool) {}
}

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionCommandValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionCommand_Key, TReference], e TargetActionCommandEnvironment[TReference, TMetadata]) (PatchedTargetActionCommandValue, error) {
	id := model_core.Nested(key, key.Message.Id)
	if id.Message == nil {
		return PatchedTargetActionCommandValue{}, errors.New("no target action identifier specified")
	}
	targetLabel, err := label.NewCanonicalLabel(id.Message.Label)
	if err != nil {
		return PatchedTargetActionCommandValue{}, fmt.Errorf("invalid target label: %w", err)
	}

	patchedID := model_core.Patch(e, id)
	action := e.GetTargetActionValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetAction_Key{
				Id: patchedID.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID.Patcher),
		),
	)
	commandEncoder, gotCommandEncoder := e.GetCommandEncoderObjectValue(&model_analysis_pb.CommandEncoderObject_Key{})
	commandReaders, gotCommandReaders := e.GetCommandReadersValue(&model_analysis_pb.CommandReaders_Key{})
	allBuiltinsModulesNames := e.GetBuiltinsModuleNamesValue(&model_analysis_pb.BuiltinsModuleNames_Key{})
	directoryCreationParametersMessage := e.GetDirectoryCreationParametersValue(&model_analysis_pb.DirectoryCreationParameters_Key{})
	fileCreationParametersMessage := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !action.IsSet() ||
		!allBuiltinsModulesNames.IsSet() ||
		!gotCommandEncoder ||
		!gotCommandReaders ||
		!directoryCreationParametersMessage.IsSet() ||
		!fileCreationParametersMessage.IsSet() {
		return PatchedTargetActionCommandValue{}, evaluation.ErrMissingDependency
	}

	actionDefinition := action.Message.Definition
	if actionDefinition == nil {
		return PatchedTargetActionCommandValue{}, errors.New("action definition missing")
	}

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
		model_core.Nested(action, actionDefinition.Arguments),
		/* traverser = */ func(element model_core.Message[*model_analysis_pb.Args, TReference]) (*model_core_pb.DecodableReference, error) {
			return element.Message.GetParent().GetReference(), nil
		},
		&errIterArgs,
	) {
		argsLeaf, ok := args.Message.Level.(*model_analysis_pb.Args_Leaf_)
		if !ok {
			return PatchedTargetActionCommandValue{}, errors.New("args entry is not a leaf")
		}
		var errIterAdd error
		for add := range btree.AllLeaves(
			ctx,
			c.argsAddReader,
			model_core.Nested(args, argsLeaf.Leaf.Adds),
			/* traverser = */ func(element model_core.Message[*model_analysis_pb.Args_Leaf_Add, TReference]) (*model_core_pb.DecodableReference, error) {
				return element.Message.GetParent().GetReference(), nil
			},
			&errIterAdd,
		) {
			addLeaf, ok := add.Message.Level.(*model_analysis_pb.Args_Leaf_Add_Leaf_)
			if !ok {
				return PatchedTargetActionCommandValue{}, errors.New("args.add*() entry is not a leaf")
			}

			values, err := model_starlark.DecodeValue[TReference, TMetadata](
				model_core.Nested(add, addLeaf.Leaf.Values),
				/* currentIdentifier = */ nil,
				valueDecodingOptions,
			)
			if err != nil {
				return PatchedTargetActionCommandValue{}, err
			}
			var valuesIter iter.Seq[starlark.Value]
			switch typedValues := values.(type) {
			case *model_starlark.Depset[TReference, TMetadata]:
				list, err := typedValues.ToList(thread)
				if err != nil {
					return PatchedTargetActionCommandValue{}, err
				}
				valuesIter = slices.Values(list)
			case starlark.Iterable:
				valuesIter = starlark.Elements(typedValues)
			default:
				return PatchedTargetActionCommandValue{}, errors.New("args.add*() value is not a depset or list")
			}

			// Apply the following transformation steps:
			// https://bazel.build/rules/lib/builtins/Args#add_all

			// Step 1: Each directory File item is replaced by all
			// Files recursively contained in that directory.
			if addLeaf.Leaf.ExpandDirectories {
				var expandedValues []starlark.Value
				for v := range valuesIter {
					if f, ok := v.(*model_starlark.File[TReference, TMetadata]); ok {
						var errIter error
						for child := range c.expandFileIfDirectory(e, f, &errIter) {
							expandedValues = append(expandedValues, child)
						}
						if errIter != nil {
							return PatchedTargetActionCommandValue{}, err
						}
					} else {
						expandedValues = append(expandedValues, v)
					}
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

				// The map_each function is allowed to have
				// multiple shapes. If it has a single
				// parameter, it's only called with the File.
				// If it has two parameters, it is invoked
				// with a DirectoryExpander that can be used
				// to selectively perform expansion.
				numParams, err := mapEachFunc.NumParams(thread)
				if err != nil {
					return PatchedTargetActionCommandValue{}, fmt.Errorf("unable to determine number of parameters of map_each function: %w", err)
				}
				var mapEachFuncArgs starlark.Tuple
				switch numParams {
				case 1:
					mapEachFuncArgs = make(starlark.Tuple, 1)
				case 2:
					mapEachFuncArgs = make(starlark.Tuple, 2)
					mapEachFuncArgs[1] = &directoryExpander[TReference, TMetadata]{
						computer:    c,
						environment: e,
					}
				default:
					return PatchedTargetActionCommandValue{}, errors.New("map_each function should have 1 or 2 parameters")
				}

				for v := range valuesIter {
					mapEachFuncArgs[0] = v
					returnValue, err := starlark.Call(
						thread,
						mapEachFunc,
						mapEachFuncArgs,
						nil,
					)
					if err != nil {
						if !errors.Is(err, evaluation.ErrMissingDependency) {
							var evalErr *starlark.EvalError
							if errors.As(err, &evalErr) {
								return PatchedTargetActionCommandValue{}, errors.New(evalErr.Backtrace())
							}
						}
						return PatchedTargetActionCommandValue{}, err
					}
					var s []string
					if err := unpack.IfNotNone(unpack.Or([]unpack.UnpackerInto[[]string]{
						unpack.Singleton(unpack.String),
						unpack.List(unpack.String),
					})).UnpackInto(thread, returnValue, &s); err != nil {
						return PatchedTargetActionCommandValue{}, fmt.Errorf("failed to unpack map function return value: %w", err)
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
							return PatchedTargetActionCommandValue{}, err
						}
					case model_starlark.Label[TReference, TMetadata]:
						s = typedV.String()
					default:
						return PatchedTargetActionCommandValue{}, fmt.Errorf("argument value is of type %#v, while a string, File or Label were expected", typedV.Type())
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
					return PatchedTargetActionCommandValue{}, err
				}
			}

			formatEachPrefix, formatEachSuffix, err := splitArgsTemplate(addLeaf.Leaf.FormatEach)
			if err != nil {
				return PatchedTargetActionCommandValue{}, fmt.Errorf("invalid value for args.add_*(format_each=%#v): %w", addLeaf.Leaf.FormatEach, err)
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
							return PatchedTargetActionCommandValue{}, err
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
						return PatchedTargetActionCommandValue{}, err
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
						return PatchedTargetActionCommandValue{}, err
					}
				}
			case *model_analysis_pb.Args_Leaf_Add_Leaf_Joined_:
				formatJoinedPrefix, formatJoinedSuffix, err := splitArgsTemplate(style.Joined.FormatJoined)
				if err != nil {
					return PatchedTargetActionCommandValue{}, fmt.Errorf("invalid value for args.add_*(format_joined=%#v): %w", style.Joined.FormatJoined, err)
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
					return PatchedTargetActionCommandValue{}, err
				}
			default:
				return PatchedTargetActionCommandValue{}, errors.New("unknown args.add*() style")
			}
		}
		if errIterAdd != nil {
			return PatchedTargetActionCommandValue{}, errIterAdd
		}
	}
	if errIterArgs != nil {
		return PatchedTargetActionCommandValue{}, errIterArgs
	}
	argumentsList, err := argumentsBuilder.FinalizeList()
	if err != nil {
		return PatchedTargetActionCommandValue{}, err
	}

	// TODO: Also respect use_default_shell_env.
	environmentVariablesList := model_core.PatchList(e, model_core.Nested(action, actionDefinition.Env))

	// The provided output path pattern is relative to the output
	// directory of the current configuration and package. Prepend
	// pathname components to make it relative to the input root.
	packageRelativeOutputPathPatternChildren, err := model_command.PathPatternGetChildren(
		ctx,
		commandReaders.PathPatternChildren,
		model_core.Nested(action, actionDefinition.OutputPathPattern),
	)
	if err != nil {
		return PatchedTargetActionCommandValue{}, err
	}
	outputPathPatternChildren := model_core.Patch(e, packageRelativeOutputPathPatternChildren)
	inlinedTreeOptions := c.getInlinedTreeOptions()
	targetPackage := targetLabel.GetCanonicalPackage()
	packageComponents := strings.FieldsFunc(targetPackage.GetPackagePath(), func(r rune) bool { return r == '/' })
	for i := len(packageComponents) - 1; i >= 0; i-- {
		outputPathPatternChildren, err = model_command.PrependDirectoryToPathPatternChildren(
			packageComponents[i],
			outputPathPatternChildren,
			commandEncoder,
			inlinedTreeOptions,
			e,
		)
		if err != nil {
			return PatchedTargetActionCommandValue{}, err
		}
	}
	configurationReferenceComponent, err := model_starlark.ConfigurationReferenceToComponent(model_core.Nested(id, id.Message.ConfigurationReference))
	if err != nil {
		return PatchedTargetActionCommandValue{}, err
	}
	for _, component := range []string{
		targetPackage.GetCanonicalRepo().String(),
		model_starlark.ComponentStrExternal,
		model_starlark.ComponentStrBin,
		configurationReferenceComponent,
		model_starlark.ComponentStrBazelOut,
	} {
		outputPathPatternChildren, err = model_command.PrependDirectoryToPathPatternChildren(
			component,
			outputPathPatternChildren,
			commandEncoder,
			inlinedTreeOptions,
			e,
		)
		if err != nil {
			return PatchedTargetActionCommandValue{}, err
		}
	}

	// Construct the command of the action.
	command, err := inlinedtree.Build(
		inlinedtree.CandidateList[*model_command_pb.Command, TMetadata]{
			// Fields that should always be inlined into the
			// Command message.
			{
				ExternalMessage: model_core.NewSimplePatchedMessage[TMetadata]((proto.Message)(nil)),
				ParentAppender: func(
					command model_core.PatchedMessage[*model_command_pb.Command, TMetadata],
					externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
				) {
					command.Message.DirectoryCreationParameters = directoryCreationParametersMessage.Message.DirectoryCreationParameters
					command.Message.FileCreationParameters = fileCreationParametersMessage.Message.FileCreationParameters
					command.Message.WorkingDirectory = (*path.Trace)(nil).GetUNIXString()
				},
			},
			// Fields that can be stored externally if needed.
			{
				ExternalMessage: model_core.NewPatchedMessage(
					(proto.Message)(nil),
					argumentsList.Patcher,
				),
				ParentAppender: func(
					command model_core.PatchedMessage[*model_command_pb.Command, TMetadata],
					externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
				) {
					// TODO: This should push out the
					// arguments if they get too big.
					command.Message.Arguments = argumentsList.Message
				},
			},
			{
				ExternalMessage: model_core.NewPatchedMessage(
					(proto.Message)(nil),
					environmentVariablesList.Patcher,
				),
				ParentAppender: func(
					command model_core.PatchedMessage[*model_command_pb.Command, TMetadata],
					externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
				) {
					// TODO: This should push out
					// the environment variables if
					// they get too big.
					command.Message.EnvironmentVariables = environmentVariablesList.Message
				},
			},
			{
				ExternalMessage: model_core.NewPatchedMessage[proto.Message](
					outputPathPatternChildren.Message,
					outputPathPatternChildren.Patcher,
				),
				Encoder: commandEncoder,
				ParentAppender: func(
					command model_core.PatchedMessage[*model_command_pb.Command, TMetadata],
					externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
				) {
					command.Message.OutputPathPattern = model_command.GetPathPatternWithChildren(
						outputPathPatternChildren,
						externalObject,
						command.Patcher,
						e,
					)
				},
			},
		},
		inlinedTreeOptions,
	)
	if err != nil {
		return PatchedTargetActionCommandValue{}, err
	}
	createdCommand, err := model_core.MarshalAndEncodePatchedMessage(
		command,
		referenceFormat,
		commandEncoder,
	)

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetActionCommand_Value{
			CommandReference: patcher.CaptureAndAddDecodableReference(createdCommand, e),
		},
		model_core.MapReferenceMetadataToWalkers(patcher),
	), nil
}

// directoryExpander implements the DirectoryExpander type that is
// provided to the "map_each" callback used by Args.add_*(). It provides
// the ability to expand directories to a list of files.
type directoryExpander[TReference object.BasicReference, TMetadata BaseComputerReferenceMetadata] struct {
	computer    *baseComputer[TReference, TMetadata]
	environment expandFileIfDirectoryEnvironment[TReference, TMetadata]
}

var _ starlark.HasAttrs = (*directoryExpander[object.LocalReference, BaseComputerReferenceMetadata])(nil)

func (directoryExpander[TReference, TMetadata]) String() string {
	return "<DirectoryExpander>"
}

func (directoryExpander[TReference, TMetadata]) Type() string {
	return "DirectoryExpander"
}

func (directoryExpander[TReference, TMetadata]) Freeze() {}

func (directoryExpander[TReference, TMetadata]) Truth() starlark.Bool {
	return starlark.True
}

func (directoryExpander[TReference, TMetadata]) Hash(thread *starlark.Thread) (uint32, error) {
	return 0, errors.New("DirectoryExpander cannot be hashed")
}

func (de *directoryExpander[TReference, TMetadata]) Attr(thread *starlark.Thread, name string) (starlark.Value, error) {
	switch name {
	case "expand":
		return starlark.NewBuiltin("DirectoryExpander.expand", de.doExpand), nil
	default:
		return nil, nil
	}
}

var directoryExpanderAttrNames = []string{
	"expand",
}

func (directoryExpander[TReference, TMetadata]) AttrNames() []string {
	return directoryExpanderAttrNames
}

func (de *directoryExpander[TReference, TMetadata]) doExpand(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var file *model_starlark.File[TReference, TMetadata]
	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"file", unpack.Bind(thread, &file, unpack.Type[*model_starlark.File[TReference, TMetadata]]("File")),
	); err != nil {
		return nil, err
	}

	var files []starlark.Value
	var errIter error
	for child := range de.computer.expandFileIfDirectory(de.environment, file, &errIter) {
		files = append(files, child)
	}
	if errIter != nil {
		return nil, errIter
	}
	return starlark.NewList(files), nil
}
