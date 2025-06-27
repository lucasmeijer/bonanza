package label_test

import (
	"testing"

	"bonanza.build/pkg/label"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestTargetName(t *testing.T) {
	t.Run("GetSibling", func(t *testing.T) {
		assert.Equal(
			t,
			"A/B",
			util.Must(label.NewTargetName("a")).
				GetSibling(util.Must(label.NewTargetName("A/B"))).
				String(),
		)
		assert.Equal(
			t,
			"a/A/B",
			util.Must(label.NewTargetName("a/b")).
				GetSibling(util.Must(label.NewTargetName("A/B"))).
				String(),
		)
		assert.Equal(
			t,
			"a/b/A/B",
			util.Must(label.NewTargetName("a/b/c")).
				GetSibling(util.Must(label.NewTargetName("A/B"))).
				String(),
		)
	})

	t.Run("ToComponents", func(t *testing.T) {
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
			},
			util.Must(label.NewTargetName("a")).ToComponents(),
		)
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
				path.MustNewComponent("b"),
			},
			util.Must(label.NewTargetName("a/b")).ToComponents(),
		)
		assert.Equal(
			t,
			[]path.Component{
				path.MustNewComponent("a"),
				path.MustNewComponent("b"),
				path.MustNewComponent("c"),
			},
			util.Must(label.NewTargetName("a/b/c")).ToComponents(),
		)
	})
}
