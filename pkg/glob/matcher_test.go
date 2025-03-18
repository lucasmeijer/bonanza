package glob_test

import (
	"testing"

	"github.com/buildbarn/bonanza/pkg/glob"
	"github.com/stretchr/testify/require"
)

func TestMatcher(t *testing.T) {
	t.Run("FileExtensions", func(t *testing.T) {
		nfa, err := glob.NewNFAFromPatterns(
			[]string{
				"*.h",
				"*.H",
				"*.h++",
				"*.hh",
				"*.hpp",
				"*.hxx",
				"*.inc",
				"*.inl",
				"*.ipp",
				"*.tcc",
				"*.tlh",
				"*.tli",
			},
			nil,
		)
		require.NoError(t, err)

		t.Run("UnrelatedDirectory", func(t *testing.T) {
			var m glob.Matcher
			m.Initialize(nfa)
			require.True(t, m.WriteRune('a'))
			require.False(t, m.IsMatch())
			require.False(t, m.WriteRune('/'))
		})

		t.Run("Match", func(t *testing.T) {
			var m glob.Matcher
			m.Initialize(nfa)
			require.True(t, m.WriteRune('f'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('.'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('h'))
			require.True(t, m.IsMatch())
			require.False(t, m.WriteRune('/'))
		})
	})

	t.Run("StarStarSlash", func(t *testing.T) {
		nfa, err := glob.NewNFAFromPatterns(
			[]string{
				"**/foo.h",
			},
			nil,
		)
		require.NoError(t, err)

		t.Run("AtRoot", func(t *testing.T) {
			var m glob.Matcher
			m.Initialize(nfa)
			require.True(t, m.WriteRune('f'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('.'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('h'))
			require.True(t, m.IsMatch())
		})

		t.Run("InDirectory", func(t *testing.T) {
			var m glob.Matcher
			m.Initialize(nfa)
			require.True(t, m.WriteRune('b'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('a'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('r'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('/'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('f'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('o'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('.'))
			require.False(t, m.IsMatch())
			require.True(t, m.WriteRune('h'))
			require.True(t, m.IsMatch())
		})
	})
}
