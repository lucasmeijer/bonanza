package formatted_test

import (
	"strings"
	"testing"

	"github.com/buildbarn/bonanza/pkg/bazelclient/formatted"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePlainText(t *testing.T) {
	t.Run("Text", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WritePlainText(
			formatted.Text("Hello, world!"),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 13, n)
		assert.Equal(t, "Hello, world!", b.String())
	})

	t.Run("Textf", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WritePlainText(
			formatted.Textf("%d + %d = %d", 2, 2, 4),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 9, n)
		assert.Equal(t, "2 + 2 = 4", b.String())
	})

	t.Run("Join", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WritePlainText(
			formatted.Join(
				formatted.Text("Hello"),
				formatted.Text(", "),
				formatted.Text("world"),
				formatted.Text("!"),
			),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 13, n)
		assert.Equal(t, "Hello, world!", b.String())
	})

	t.Run("Attr", func(t *testing.T) {
		// Formatting should be suppressed when generating plain
		// text output.
		var b strings.Builder
		n, err := formatted.WritePlainText(
			formatted.Bold(formatted.Red(formatted.Text("Danger"))),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 6, n)
		assert.Equal(t, "Danger", b.String())
	})
}
