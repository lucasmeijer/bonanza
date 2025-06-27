package filesystem_test

import (
	"testing"

	model_core "bonanza.build/pkg/model/core"
	model_filesystem "bonanza.build/pkg/model/filesystem"
	model_core_pb "bonanza.build/pkg/proto/model/core"
	model_filesystem_pb "bonanza.build/pkg/proto/model/filesystem"
	"bonanza.build/pkg/storage/dag"

	"github.com/buildbarn/bb-storage/pkg/testutil"
	"github.com/stretchr/testify/require"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestNewCreatedDirectoryBare(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		cd, err := model_filesystem.NewCreatedDirectoryBare(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_filesystem_pb.DirectoryContents{
					Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
						LeavesInline: &model_filesystem_pb.Leaves{},
					},
				},
			),
		)
		require.NoError(t, err)
		testutil.RequireEqualProto(
			t,
			&wrapperspb.UInt32Value{Value: 0},
			cd.MaximumSymlinkEscapementLevels,
		)
	})

	t.Run("RootSymlinkNotEscaping", func(t *testing.T) {
		cd, err := model_filesystem.NewCreatedDirectoryBare(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_filesystem_pb.DirectoryContents{
					Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
						LeavesInline: &model_filesystem_pb.Leaves{
							Symlinks: []*model_filesystem_pb.SymlinkNode{{
								Name:   "hello",
								Target: "target",
							}},
						},
					},
				},
			),
		)
		require.NoError(t, err)
		testutil.RequireEqualProto(
			t,
			&wrapperspb.UInt32Value{Value: 0},
			cd.MaximumSymlinkEscapementLevels,
		)
	})

	t.Run("RootSymlinkEscapingSingleLevel", func(t *testing.T) {
		cd, err := model_filesystem.NewCreatedDirectoryBare(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_filesystem_pb.DirectoryContents{
					Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
						LeavesInline: &model_filesystem_pb.Leaves{
							Symlinks: []*model_filesystem_pb.SymlinkNode{{
								Name:   "hello",
								Target: "../target",
							}},
						},
					},
				},
			),
		)
		require.NoError(t, err)
		testutil.RequireEqualProto(
			t,
			&wrapperspb.UInt32Value{Value: 1},
			cd.MaximumSymlinkEscapementLevels,
		)
	})

	t.Run("RootSymlinkAbsolute", func(t *testing.T) {
		cd, err := model_filesystem.NewCreatedDirectoryBare(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_filesystem_pb.DirectoryContents{
					Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
						LeavesInline: &model_filesystem_pb.Leaves{
							Symlinks: []*model_filesystem_pb.SymlinkNode{{
								Name:   "hello",
								Target: "/target",
							}},
						},
					},
				},
			),
		)
		require.NoError(t, err)
		require.Nil(t, cd.MaximumSymlinkEscapementLevels)
	})

	t.Run("DirectoryEscapingMultipleLevels", func(t *testing.T) {
		cd, err := model_filesystem.NewCreatedDirectoryBare(
			model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](
				&model_filesystem_pb.DirectoryContents{
					Leaves: &model_filesystem_pb.DirectoryContents_LeavesInline{
						LeavesInline: &model_filesystem_pb.Leaves{},
					},
					Directories: []*model_filesystem_pb.DirectoryNode{{
						Name: "hello",
						Directory: &model_filesystem_pb.Directory{
							Contents: &model_filesystem_pb.Directory_ContentsExternal{
								ContentsExternal: &model_filesystem_pb.DirectoryReference{
									Reference: &model_core_pb.DecodableReference{
										Reference: &model_core_pb.Reference{Index: 1},
									},
									MaximumSymlinkEscapementLevels: &wrapperspb.UInt32Value{Value: 3},
								},
							},
						},
					}},
				},
			),
		)
		require.NoError(t, err)
		testutil.RequireEqualProto(
			t,
			&wrapperspb.UInt32Value{Value: 2},
			cd.MaximumSymlinkEscapementLevels,
		)
	})
}
