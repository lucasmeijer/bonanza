package analysis_test

import (
	"testing"

	"github.com/buildbarn/bb-storage/pkg/testutil"
	model_analysis "github.com/buildbarn/bonanza/pkg/model/analysis"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/stretchr/testify/require"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.uber.org/mock/gomock"
)

// emptyLeaves can be embedded into DirectoryContents messages to
// indicate that a directory contains no files or symlinks.
var emptyLeaves = &model_filesystem_pb.DirectoryContents_LeavesInline{
	LeavesInline: &model_filesystem_pb.Leaves{},
}

// singleChildDirectoryContents can be used to construct a directory
// that only contains a single child that is also a directory.
func singleChildDirectoryContents(name string, childContents *model_filesystem_pb.DirectoryContents) *model_filesystem_pb.DirectoryContents {
	return &model_filesystem_pb.DirectoryContents{
		Directories: []*model_filesystem_pb.DirectoryNode{{
			Name: name,
			Directory: &model_filesystem_pb.Directory{
				Contents: &model_filesystem_pb.Directory_ContentsInline{
					ContentsInline: childContents,
				},
			},
		}},
		Leaves: emptyLeaves,
	}
}

func TestFileRoot(t *testing.T) {
	ctrl, ctx := gomock.WithContext(t.Context(), t)
	bct := newBaseComputerTester(ctrl)

	t.Run("MissingFile", func(t *testing.T) {
		// Request needs to contain a Starlark File object.
		e := NewMockFileRootEnvironmentForTesting(ctrl)

		_, err := bct.computer.ComputeFileRootValue(
			ctx,
			model_core.NewSimpleMessage[model_core.CreatedObjectTree](
				&model_analysis_pb.FileRoot_Key{
					DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
				},
			),
			e,
		)
		require.EqualError(t, err, "no file provided")
	})

	t.Run("BadLabel", func(t *testing.T) {
		// Label in Starlark File object needs to be well formed.
		e := NewMockFileRootEnvironmentForTesting(ctrl)

		_, err := bct.computer.ComputeFileRootValue(
			ctx,
			model_core.NewSimpleMessage[model_core.CreatedObjectTree](
				&model_analysis_pb.FileRoot_Key{
					DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
					File: &model_starlark_pb.File{
						Label: "this is not a valid label",
						Type:  model_starlark_pb.File_FILE,
					},
				},
			),
			e,
		)
		require.ErrorContains(t, err, "invalid file label: ")
	})

	t.Run("NoOwner", func(t *testing.T) {
		t.Run("NonFile", func(t *testing.T) {
			// Bazel only has the ability to create File
			// objects referring to individual source files.
			// It is not possible to refer to directories.
			// It is only possible to call glob(), which
			// (when provided to a label list attribute)
			// ends up creating a list of File objects.
			//
			// Eventually we might want to provide an
			// extension to support the creation of "source
			// directories", as this can be advantageous for
			// things like prebuilt SDKs. For now we stick
			// to the same API as Bazel.
			e := NewMockFileRootEnvironmentForTesting(ctrl)

			_, err := bct.computer.ComputeFileRootValue(
				ctx,
				model_core.NewSimpleMessage[model_core.CreatedObjectTree](
					&model_analysis_pb.FileRoot_Key{
						DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
						File: &model_starlark_pb.File{
							Label: "@@myrepo+//a/b:c",
							Type:  model_starlark_pb.File_DIRECTORY,
						},
					},
				),
				e,
			)
			require.EqualError(t, err, "source files can only be regular files")
		})

		t.Run("NonExistent", func(t *testing.T) {
			// Labels that refer to non-existent files
			// should cause the creation of a file root to
			// fail.
			e := NewMockFileRootEnvironmentForTesting(ctrl)
			bct.expectGetDirectoryCreationParametersObjectValue(t, e)
			bct.expectGetDirectoryReadersValue(t, e)
			e.EXPECT().GetRepoValue(
				testutil.EqProto(t, &model_analysis_pb.Repo_Key{
					CanonicalRepo: "myrepo+",
				}),
			).Return(newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.Repo_Value {
				return &model_analysis_pb.Repo_Value{
					RootDirectoryReference: &model_filesystem_pb.DirectoryReference{
						Reference: attachObject(patcher, newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
							return model_core.NewProtoMarshalable(&model_filesystem_pb.DirectoryContents{
								Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
									LeavesInline: &model_filesystem_pb.Leaves{
										Files: []*model_filesystem_pb.FileNode{{
											Name:       "bar",
											Properties: &model_filesystem_pb.FileProperties{},
										}},
									},
								},
							})
						})),
						DirectoriesCount:               0,
						MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
					},
				}
			}))

			_, err := bct.computer.ComputeFileRootValue(
				ctx,
				model_core.NewSimpleMessage[model_core.CreatedObjectTree](
					&model_analysis_pb.FileRoot_Key{
						DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
						File: &model_starlark_pb.File{
							Label: "@@myrepo+//:foo",
							Type:  model_starlark_pb.File_FILE,
						},
					},
				),
				e,
			)
			require.ErrorContains(t, err, "Path does not exist")
		})

		t.Run("FileResolvesToDirectory", func(t *testing.T) {
			// If the type of the File object is a regular
			// file, the path to which it resolves should
			// also be a regular file.
			e := NewMockFileRootEnvironmentForTesting(ctrl)
			bct.expectGetDirectoryCreationParametersObjectValue(t, e)
			bct.expectGetDirectoryReadersValue(t, e)
			e.EXPECT().GetRepoValue(
				testutil.EqProto(t, &model_analysis_pb.Repo_Key{
					CanonicalRepo: "myrepo+",
				}),
			).Return(newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.Repo_Value {
				return &model_analysis_pb.Repo_Value{
					RootDirectoryReference: &model_filesystem_pb.DirectoryReference{
						Reference: attachObject(patcher, newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
							return model_core.NewProtoMarshalable(singleChildDirectoryContents(
								"foo",
								&model_filesystem_pb.DirectoryContents{
									Leaves: emptyLeaves,
								},
							))
						})),
						DirectoriesCount:               0,
						MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
					},
				}
			}))

			_, err := bct.computer.ComputeFileRootValue(
				ctx,
				model_core.NewSimpleMessage[model_core.CreatedObjectTree](
					&model_analysis_pb.FileRoot_Key{
						DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
						File: &model_starlark_pb.File{
							Label: "@@myrepo+//:foo",
							Type:  model_starlark_pb.File_FILE,
						},
					},
				),
				e,
			)
			require.EqualError(t, err, "path resolves to a directory, while a file was expected")
		})

		t.Run("SuccessSimple", func(t *testing.T) {
			// Simple scenario in which the path resolves to
			// a regular file. The resulting root should
			// only contain the specified file. Any
			// unrelated files should be removed.
			run := func(t *testing.T, directoryLayout model_analysis_pb.DirectoryLayout) model_analysis.PatchedFileRootValue {
				e := NewMockFileRootEnvironmentForTesting(ctrl)
				bct.expectGetDirectoryCreationParametersObjectValue(t, e)
				bct.expectGetDirectoryReadersValue(t, e)
				e.EXPECT().GetRepoValue(
					testutil.EqProto(t, &model_analysis_pb.Repo_Key{
						CanonicalRepo: "myrepo+",
					}),
				).Return(newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.Repo_Value {
					return &model_analysis_pb.Repo_Value{
						RootDirectoryReference: &model_filesystem_pb.DirectoryReference{
							Reference: attachObject(patcher, newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
								return model_core.NewProtoMarshalable(&model_filesystem_pb.DirectoryContents{
									Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
										LeavesInline: &model_filesystem_pb.Leaves{
											Files: []*model_filesystem_pb.FileNode{
												{
													Name:       "bar",
													Properties: &model_filesystem_pb.FileProperties{},
												},
												{
													Name:       "foo",
													Properties: &model_filesystem_pb.FileProperties{},
												},
											},
										},
									},
								})
							})),
							DirectoriesCount:               0,
							MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
						},
					}
				}))

				fileRoot, err := bct.computer.ComputeFileRootValue(
					ctx,
					model_core.NewSimpleMessage[model_core.CreatedObjectTree](
						&model_analysis_pb.FileRoot_Key{
							DirectoryLayout: directoryLayout,
							File: &model_starlark_pb.File{
								Label: "@@myrepo+//:bar",
								Type:  model_starlark_pb.File_FILE,
							},
						},
					),
					e,
				)
				require.NoError(t, err)
				return fileRoot
			}

			t.Run("InputRoot", func(t *testing.T) {
				// When source files are placed in input
				// roots, they should be named
				// "external/${repo}/${file}".
				fileRoot := run(t, model_analysis_pb.DirectoryLayout_INPUT_ROOT)
				requireEqualPatchedMessage(t, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
					return &model_analysis_pb.FileRoot_Value{
						RootDirectory: singleChildDirectoryContents(
							"external",
							singleChildDirectoryContents(
								"myrepo+",
								&model_filesystem_pb.DirectoryContents{
									Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
										LeavesInline: &model_filesystem_pb.Leaves{
											Files: []*model_filesystem_pb.FileNode{
												{
													Name:       "bar",
													Properties: &model_filesystem_pb.FileProperties{},
												},
											},
										},
									},
								},
							),
						),
					}
				}, fileRoot)
				fileRoot.Discard()
			})

			t.Run("Runfiles", func(t *testing.T) {
				// When source files are placed in
				// runfiles directories, they should be
				// named "${repo}/${file}".
				fileRoot := run(t, model_analysis_pb.DirectoryLayout_RUNFILES)
				requireEqualPatchedMessage(t, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
					return &model_analysis_pb.FileRoot_Value{
						RootDirectory: singleChildDirectoryContents(
							"myrepo+",
							&model_filesystem_pb.DirectoryContents{
								Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
									LeavesInline: &model_filesystem_pb.Leaves{
										Files: []*model_filesystem_pb.FileNode{
											{
												Name:       "bar",
												Properties: &model_filesystem_pb.FileProperties{},
											},
										},
									},
								},
							},
						),
					}
				}, fileRoot)
				fileRoot.Discard()
			})
		})
	})

	t.Run("Action", func(t *testing.T) {
		// TODO!
	})

	t.Run("ExpandTemplate", func(t *testing.T) {
		// TODO: Test error cases.

		t.Run("Success", func(t *testing.T) {
			// Simulate the computation of the output of:
			//
			//     ctx.actions.expand_template(
			//         template = File("@@myrepo+//:template"),
			//         output = File("@@myrepo+//:output"),
			//         substitutions = {
			//             "{{first_name}}": "Albert",
			//             "{{last_name}}": "Einstein",
			//         },
			//         is_executable = True,
			//     )
			e := NewMockFileRootEnvironmentForTesting(ctrl)
			bct.expectCaptureExistingObject(e)
			bct.expectGetDirectoryCreationParametersObjectValue(t, e)
			bct.expectGetDirectoryReadersValue(t, e)
			bct.expectGetFileCreationParametersObjectValue(t, e)
			bct.expectGetFileReaderValue(t, e)
			configuration := newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
				return model_core.NewProtoListMarshalable([]*model_analysis_pb.BuildSettingOverride{{
					Level: &model_analysis_pb.BuildSettingOverride_Leaf_{
						Leaf: &model_analysis_pb.BuildSettingOverride_Leaf{
							Label: "@@bazel_tools+//command_line_option:platforms",
							Value: &model_starlark_pb.Value{
								Kind: &model_starlark_pb.Value_List{
									List: &model_starlark_pb.List{
										Elements: []*model_starlark_pb.List_Element{{
											Level: &model_starlark_pb.List_Element_Leaf{
												Leaf: &model_starlark_pb.Value{
													Kind: &model_starlark_pb.Value_Label{
														Label: "@@platforms+//host",
													},
												},
											},
										}},
									},
								},
							},
						},
					},
				}})
			})
			e.EXPECT().GetTargetOutputValue(
				eqPatchedMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.TargetOutput_Key {
					return &model_analysis_pb.TargetOutput_Key{
						Label:                  "@@myrepo+//:generate",
						ConfigurationReference: attachObject(patcher, configuration),
						PackageRelativePath:    "output",
					}
				}),
			).Return(newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.TargetOutput_Value {
				return &model_analysis_pb.TargetOutput_Value{
					Definition: &model_analysis_pb.TargetOutputDefinition{
						Source: &model_analysis_pb.TargetOutputDefinition_ExpandTemplate_{
							ExpandTemplate: &model_analysis_pb.TargetOutputDefinition_ExpandTemplate{
								Template: &model_starlark_pb.File{
									Label: "@@myrepo+//:template",
									Type:  model_starlark_pb.File_FILE,
								},
								IsExecutable: true,
								Substitutions: []*model_analysis_pb.TargetOutputDefinition_ExpandTemplate_Substitution{
									{Needle: []byte("{{first_name}}"), Replacement: []byte("Albert")},
									{Needle: []byte("{{last_name}}"), Replacement: []byte("Einstein")},
								},
							},
						},
					},
				}
			}))
			e.EXPECT().GetFileRootValue(
				eqPatchedMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Key {
					return &model_analysis_pb.FileRoot_Key{
						DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
						File: &model_starlark_pb.File{
							Label: "@@myrepo+//:template",
							Type:  model_starlark_pb.File_FILE,
						},
					}
				}),
			).Return(newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
				return &model_analysis_pb.FileRoot_Value{
					RootDirectory: singleChildDirectoryContents(
						"external",
						singleChildDirectoryContents(
							"myrepo+",
							&model_filesystem_pb.DirectoryContents{
								Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
									LeavesInline: &model_filesystem_pb.Leaves{
										Files: []*model_filesystem_pb.FileNode{
											{
												Name: "template",
												Properties: &model_filesystem_pb.FileProperties{
													Contents: &model_filesystem_pb.FileContents{
														Level: &model_filesystem_pb.FileContents_ChunkReference{
															ChunkReference: attachObject(patcher, newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
																return model_core.NewRawMarshalable([]byte("{{first_name}} {{last_name}}"))
															})),
														},
														TotalSizeBytes: 28,
													},
												},
											},
										},
									},
								},
							},
						),
					),
				}
			}))
			expandedTemplateFile := NewMockFileReadWriter(ctrl)
			bct.filePool.EXPECT().NewFile().Return(expandedTemplateFile, nil)
			expandedTemplateFile.EXPECT().WriteAt([]byte("Albert Einstein"), int64(0)).Return(15, nil)
			expandedTemplateFile.EXPECT().Close()

			fileRoot, err := bct.computer.ComputeFileRootValue(
				ctx,
				newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Key {
					return &model_analysis_pb.FileRoot_Key{
						DirectoryLayout: model_analysis_pb.DirectoryLayout_INPUT_ROOT,
						File: &model_starlark_pb.File{
							Label: "@@myrepo+//:output",
							Type:  model_starlark_pb.File_FILE,
							Owner: &model_starlark_pb.File_Owner{
								ConfigurationReference: attachObject(patcher, configuration),
								TargetName:             "generate",
							},
						},
					}
				}),
				e,
			)
			require.NoError(t, err)
			requireEqualPatchedMessage(t, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
				return &model_analysis_pb.FileRoot_Value{
					RootDirectory: singleChildDirectoryContents(
						"bazel-out",
						singleChildDirectoryContents(
							"Cg6Kx80o8BPYmGdgWYfRZvbKyWojQ7snQzHOx70XAwRPAAAAAAAAAA.",
							singleChildDirectoryContents(
								"bin",
								singleChildDirectoryContents(
									"external",
									singleChildDirectoryContents(
										"myrepo+",
										&model_filesystem_pb.DirectoryContents{
											Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
												LeavesInline: &model_filesystem_pb.Leaves{
													Files: []*model_filesystem_pb.FileNode{
														{
															Name: "output",
															Properties: &model_filesystem_pb.FileProperties{
																Contents: &model_filesystem_pb.FileContents{
																	Level: &model_filesystem_pb.FileContents_ChunkReference{
																		ChunkReference: attachObject(patcher, newObject(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) model_core.Marshalable {
																			return model_core.NewRawMarshalable([]byte("Albert Einstein"))
																		})),
																	},
																	TotalSizeBytes: 15,
																},
																IsExecutable: true,
															},
														},
													},
												},
											},
										},
									),
								),
							),
						),
					),
				}
			}, fileRoot)
			fileRoot.Discard()
		})
	})

	t.Run("StaticPackageDirectory", func(t *testing.T) {
		// TODO!
	})

	t.Run("Symlink", func(t *testing.T) {
		// TODO!
	})
}
