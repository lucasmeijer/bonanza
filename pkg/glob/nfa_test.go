package glob_test

import (
	"testing"

	"bonanza.build/pkg/glob"

	"github.com/stretchr/testify/require"
)

func TestPredeclaredNFAs(t *testing.T) {
	require.Equal(t, []byte{0x80}, glob.NFAMatchingNothing.Bytes())
	require.Equal(
		t,
		[]byte{
			/*      */ 0x88,
			/* ꘎꘎/  */ 0x84,
			/* ꘎꘎/꘎ */ 0x81,
		},
		glob.NFAMatchingEverything.Bytes(),
	)
}

func TestNewNFAFromPatterns(t *testing.T) {
	t.Run("Failure", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{""}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"\": pathname components cannot be empty")
		})

		t.Run("TooManyStars", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{"***"}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"***\": \"***\" has no special meaning")
		})

		t.Run("StarStarWithinFilename1", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{"a**"}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"a**\": \"**\" can not be placed inside a component")
		})

		t.Run("StarStarWithinFilename2", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{"**a"}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"**a\": \"**\" can not be placed inside a component")
		})

		t.Run("RedundantStarStarsMiddle", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{"a/**/**/b"}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"a/**/**/b\": redundant \"**\"")
		})

		t.Run("RedundantStarStarsEnd", func(t *testing.T) {
			_, err := glob.NewNFAFromPatterns([]string{"a/**/**"}, nil)
			require.EqualError(t, err, "invalid \"includes\" pattern \"a/**/**\": redundant \"**\"")
		})
	})

	t.Run("Success", func(t *testing.T) {
		testNFAEqualBytes := func(t *testing.T, includes, excludes []string, expectedBytes []byte) {
			nfa1, err := glob.NewNFAFromPatterns(includes, excludes)
			require.NoError(t, err)
			actualBytes1 := nfa1.Bytes()
			require.Equal(t, expectedBytes, actualBytes1)

			nfa2, err := glob.NewNFAFromBytes(expectedBytes)
			require.NoError(t, err)
			actualBytes2 := nfa2.Bytes()
			require.Equal(t, expectedBytes, actualBytes2)
		}

		t.Run("Empty", func(t *testing.T) {
			testNFAEqualBytes(t, nil, nil, []byte{0x80})
		})

		t.Run("LiteralFilename", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{"hello"},
				nil,
				[]byte{
					/*       */ 0x90, 'h',
					/* h     */ 0x90, 'e',
					/* he    */ 0x90, 'l',
					/* hel   */ 0x90, 'l',
					/* hell  */ 0x90, 'o',
					/* hello */ 0x81,
				},
			)
		})

		t.Run("Star", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{"*"},
				nil,
				[]byte{
					/*   */ 0x84,
					/* * */ 0x81,
				},
			)
		})

		t.Run("MultipleStars", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{"*a*b*c*"},
				nil,
				[]byte{
					/*         */ 0x84,
					/* ꘎       */ 0x90, 'a',
					/* ꘎a      */ 0x84,
					/* ꘎a꘎     */ 0x90, 'b',
					/* ꘎a꘎b    */ 0x84,
					/* ꘎a꘎b꘎   */ 0x90, 'c',
					/* ꘎a꘎b꘎c  */ 0x84,
					/* ꘎a꘎b꘎c꘎ */ 0x81,
				},
			)
		})

		t.Run("MultipleStarStars", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{"a/**/b/**/c"},
				nil,
				[]byte{
					/*             */ 0x90, 'a',
					/* a           */ 0x90, '/',
					/* a/          */ 0x88,
					/* a/꘎꘎/       */ 0x90, 'b',
					/* a/꘎꘎/b      */ 0x90, '/',
					/* a/꘎꘎/b/     */ 0x88,
					/* a/꘎꘎/b/꘎꘎/  */ 0x90, 'c',
					/* a/꘎꘎/b/꘎꘎/c */ 0x81,
				},
			)
		})

		t.Run("StarStar", func(t *testing.T) {
			// We only provide a state transition for "**/",
			// not a bare "**". Solve this by converting it
			// to "**/*. For consistency with Bazel, the
			// root directory should not be matched.
			testNFAEqualBytes(
				t,
				[]string{"**"},
				nil,
				[]byte{
					/*      */ 0x88,
					/* ꘎꘎/  */ 0x84,
					/* ꘎꘎/꘎ */ 0x81,
				},
			)
		})

		t.Run("StarStarAtEnd", func(t *testing.T) {
			// If a pattern ends with "/**", we translate it
			// to "/**/*". For consistency with Bazel, the
			// final directory should also be matched.
			testNFAEqualBytes(
				t,
				[]string{"hello/**"},
				nil,
				[]byte{
					/*            */ 0x90, 'h',
					/* h          */ 0x90, 'e',
					/* he         */ 0x90, 'l',
					/* hel        */ 0x90, 'l',
					/* hell       */ 0x90, 'o',
					/* hello      */ 0x91, '/',
					/* hello/     */ 0x88,
					/* hello/꘎꘎/  */ 0x84,
					/* hello/꘎꘎/꘎ */ 0x81,
				},
			)
		})

		t.Run("Prefix", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{
					"good",
					"goodbye",
				},
				nil,
				[]byte{
					/*         */ 0x90, 'g',
					/* g       */ 0x90, 'o',
					/* go      */ 0x90, 'o',
					/* goo     */ 0x90, 'd',
					/* good    */ 0x91, 'b',
					/* goodb   */ 0x90, 'y',
					/* goodby  */ 0x90, 'e',
					/* goodbye */ 0x81,
				},
			)
		})

		t.Run("Branch1", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{"goodbye"},
				[]string{"good work"},
				[]byte{
					/*           */ 0x90, 'g',
					/* g         */ 0x90, 'o',
					/* go        */ 0x90, 'o',
					/* goo       */ 0x90, 'd',
					/* good      */ 0xa0, ' ', 'b',
					/* good␣     */ 0x90, 'w',
					/* goodb     */ 0x90, 'y',
					/* good␣w    */ 0x90, 'o',
					/* goodby    */ 0x90, 'e',
					/* good␣wo   */ 0x90, 'r',
					/* goodbye   */ 0x81,
					/* good␣wor  */ 0x90, 'k',
					/* good␣work */ 0x82,
				},
			)
		})

		t.Run("Branch2", func(t *testing.T) {
			// Swapping the strings around should not affect
			// the order in which states are returned.
			testNFAEqualBytes(
				t,
				[]string{"good work"},
				[]string{"goodbye"},
				[]byte{
					/*           */ 0x90, 'g',
					/* g         */ 0x90, 'o',
					/* go        */ 0x90, 'o',
					/* goo       */ 0x90, 'd',
					/* good      */ 0xa0, ' ', 'b',
					/* good␣     */ 0x90, 'w',
					/* goodb     */ 0x90, 'y',
					/* good␣w    */ 0x90, 'o',
					/* goodby    */ 0x90, 'e',
					/* good␣wo   */ 0x90, 'r',
					/* goodbye   */ 0x82,
					/* good␣wor  */ 0x90, 'k',
					/* good␣work */ 0x81,
				},
			)
		})

		t.Run("FileExtensions", func(t *testing.T) {
			testNFAEqualBytes(
				t,
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
				[]byte{
					/*       */ 0x84,
					/* ꘎     */ 0x90, '.',
					/* ꘎.    */ 0x40, 0x40, 'H', 'h', 'i', 't',
					/* ꘎.H   */ 0x81,
					/* ꘎.h   */ 0x40, 0x41, '+', 'h', 'p', 'x',
					/* ꘎.i   */ 0xa0, 'n', 'p',
					/* ꘎.t   */ 0xa0, 'c', 'l',
					/* ꘎.h+  */ 0x90, '+',
					/* ꘎.hh  */ 0x81,
					/* ꘎.hp  */ 0x90, 'p',
					/* ꘎.hx  */ 0x90, 'x',
					/* ꘎.in  */ 0xa0, 'c', 'l',
					/* ꘎.ip  */ 0x90, 'p',
					/* ꘎.tc  */ 0x90, 'c',
					/* ꘎.tl  */ 0xa0, 'h', 'i',
					/* ꘎.h++ */ 0x81,
					/* ꘎.hpp */ 0x81,
					/* ꘎.hxx */ 0x81,
					/* ꘎.inc */ 0x81,
					/* ꘎.inl */ 0x81,
					/* ꘎.ipp */ 0x81,
					/* ꘎.tcc */ 0x81,
					/* ꘎.tlh */ 0x81,
					/* ꘎.tli */ 0x81,
				},
			)
		})
	})
}

func TestNewNFAFromSuffixes(t *testing.T) {
	t.Run("Failure", func(t *testing.T) {
		t.Run("Slashes", func(t *testing.T) {
			_, err := glob.NewNFAFromSuffixes([]string{"/foo"})
			require.EqualError(t, err, "filename suffix \"/foo\" contains slashes")
		})
	})

	t.Run("Success", func(t *testing.T) {
		testNFAEqualBytes := func(t *testing.T, suffixes []string, expectedBytes []byte) {
			nfa1, err := glob.NewNFAFromSuffixes(suffixes)
			require.NoError(t, err)
			actualBytes1 := nfa1.Bytes()
			require.Equal(t, expectedBytes, actualBytes1)

			nfa2, err := glob.NewNFAFromBytes(expectedBytes)
			require.NoError(t, err)
			actualBytes2 := nfa2.Bytes()
			require.Equal(t, expectedBytes, actualBytes2)
		}

		t.Run("Empty", func(t *testing.T) {
			testNFAEqualBytes(t, nil, []byte{0x80})
		})

		t.Run("Everything", func(t *testing.T) {
			// An empty suffix should match any path.
			testNFAEqualBytes(
				t,
				[]string{""},
				[]byte{
					/*      */ 0x88,
					/* ꘎꘎/  */ 0x84,
					/* ꘎꘎/꘎ */ 0x81,
				},
			)
		})

		t.Run("Extensions", func(t *testing.T) {
			testNFAEqualBytes(
				t,
				[]string{".c", ".h"},
				[]byte{
					/*        */ 0x88,
					/* ꘎꘎/    */ 0x84,
					/* ꘎꘎/꘎   */ 0x90, '.',
					/* ꘎꘎/꘎.  */ 0xa0, 'c', 'h',
					/* ꘎꘎/꘎.c */ 0x81,
					/* ꘎꘎/꘎.h */ 0x81,
				},
			)
		})
	})
}
