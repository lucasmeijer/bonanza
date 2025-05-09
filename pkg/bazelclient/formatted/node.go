package formatted

import (
	"io"
)

// Node of formatted text that can be written either as plain text, or
// as a string containing VT100 escape sequences.
type Node interface {
	writePlainText(w io.StringWriter) (int, error)
	writeVT100(w io.StringWriter, attributesState *vt100AttributesState) (int, error)
}
