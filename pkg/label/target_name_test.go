package label_test

import (
	"testing"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/label"
	"github.com/stretchr/testify/assert"
)

func TestTargetName(t *testing.T) {
	t.Run("GetSibling", func(t *testing.T) {
		assert.Equal(
			t,
			"A/B",
			label.MustNewTargetName("a").
				GetSibling(label.MustNewTargetName("A/B")).
				String(),
		)
		assert.Equal(
			t,
			"a/A/B",
			label.MustNewTargetName("a/b").
				GetSibling(label.MustNewTargetName("A/B")).
				String(),
		)
		assert.Equal(
			t,
			"a/b/A/B",
			label.MustNewTargetName("a/b/c").
				GetSibling(label.MustNewTargetName("A/B")).
				String(),
		)
	})

	t.Run("ToComponents", func(t *testing.T) {
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
			},
			label.MustNewTargetName("a").ToComponents(),
		)
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
				path.MustNewComponent("b"),
			},
			label.MustNewTargetName("a/b").ToComponents(),
		)
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
				path.MustNewComponent("b"),
				path.MustNewComponent("c"),
			},
			label.MustNewTargetName("a/b/c").ToComponents(),
		)
	})
}
