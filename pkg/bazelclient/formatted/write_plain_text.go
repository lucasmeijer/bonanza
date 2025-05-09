package formatted

import (
	"io"
)

// WritePlainText writes the text contained in a node without any
// formatting being applied to it.
func WritePlainText(n Node, w io.StringWriter) (int, error) {
	return n.writePlainText(w)
}
