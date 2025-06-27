package formatted_test

import (
	"strings"
	"testing"

	"bonanza.build/pkg/bazelclient/formatted"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVT100(t *testing.T) {
	t.Run("Text", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WriteVT100(
			formatted.Text("Hello, world!"),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 13, n)
		assert.Equal(t, "Hello, world!", b.String())
	})

	t.Run("Textf", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WriteVT100(
			formatted.Textf("%d + %d = %d", 2, 2, 4),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 9, n)
		assert.Equal(t, "2 + 2 = 4", b.String())
	})

	t.Run("Join", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WriteVT100(
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

	t.Run("BoldRed", func(t *testing.T) {
		var b strings.Builder
		n, err := formatted.WriteVT100(
			formatted.Bold(formatted.Red(formatted.Text("Danger"))),
			&b,
		)
		require.NoError(t, err)
		assert.Equal(t, 21, n)
		assert.Equal(t, "\x1b[1;31mDanger\x1b[22;39m", b.String())
	})
}
