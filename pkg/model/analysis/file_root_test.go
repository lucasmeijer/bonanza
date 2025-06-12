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
						Reference: attachMessageObject(patcher, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_filesystem_pb.DirectoryContents {
							return &model_filesystem_pb.DirectoryContents{
								Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
									LeavesInline: &model_filesystem_pb.Leaves{
										Files: []*model_filesystem_pb.FileNode{{
											Name:       "bar",
											Properties: &model_filesystem_pb.FileProperties{},
										}},
									},
								},
							}
						}),
						DirectoriesCount:               0,
						MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
					},
				}
			}).Decay())

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
						Reference: attachMessageObject(patcher, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_filesystem_pb.DirectoryContents {
							return &model_filesystem_pb.DirectoryContents{
								Directories: []*model_filesystem_pb.DirectoryNode{{
									Name: "foo",
									Directory: &model_filesystem_pb.Directory{
										Contents: &model_filesystem_pb.Directory_ContentsInline{
											ContentsInline: &model_filesystem_pb.DirectoryContents{},
										},
									},
								}},
								Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
									LeavesInline: &model_filesystem_pb.Leaves{},
								},
							}
						}),
						DirectoriesCount:               0,
						MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
					},
				}
			}).Decay())

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
							Reference: attachMessageObject(patcher, func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_filesystem_pb.DirectoryContents {
								return &model_filesystem_pb.DirectoryContents{
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
								}
							}),
							DirectoriesCount:               0,
							MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 0},
						},
					}
				}).Decay())
				bct.expectCaptureCreatedObject(e).AnyTimes()

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
				requireEqualPatchedMessage(t, newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
					return &model_analysis_pb.FileRoot_Value{
						RootDirectory: &model_filesystem_pb.DirectoryContents{
							Directories: []*model_filesystem_pb.DirectoryNode{{
								Name: "external",
								Directory: &model_filesystem_pb.Directory{
									Contents: &model_filesystem_pb.Directory_ContentsInline{
										ContentsInline: &model_filesystem_pb.DirectoryContents{
											Directories: []*model_filesystem_pb.DirectoryNode{{
												Name: "myrepo+",
												Directory: &model_filesystem_pb.Directory{
													Contents: &model_filesystem_pb.Directory_ContentsInline{
														ContentsInline: &model_filesystem_pb.DirectoryContents{
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
													},
												},
											}},
											Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
												LeavesInline: &model_filesystem_pb.Leaves{},
											},
										},
									},
								},
							}},
							Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
								LeavesInline: &model_filesystem_pb.Leaves{},
							},
						},
					}
				}), fileRoot)
			})

			t.Run("Runfiles", func(t *testing.T) {
				// When source files are placed in
				// runfiles directories, they should be
				// named "${repo}/${file}".
				fileRoot := run(t, model_analysis_pb.DirectoryLayout_RUNFILES)
				requireEqualPatchedMessage(t, newMessage(func(patcher *model_core.ReferenceMessagePatcher[model_core.CreatedObjectTree]) *model_analysis_pb.FileRoot_Value {
					return &model_analysis_pb.FileRoot_Value{
						RootDirectory: &model_filesystem_pb.DirectoryContents{
							Directories: []*model_filesystem_pb.DirectoryNode{{
								Name: "myrepo+",
								Directory: &model_filesystem_pb.Directory{
									Contents: &model_filesystem_pb.Directory_ContentsInline{
										ContentsInline: &model_filesystem_pb.DirectoryContents{
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
									},
								},
							}},
							Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
								LeavesInline: &model_filesystem_pb.Leaves{},
							},
						},
					}
				}), fileRoot)
			})
		})
	})
}
