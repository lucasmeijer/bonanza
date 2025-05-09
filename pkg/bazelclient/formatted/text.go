package formatted

import (
	"fmt"
	"io"
)

type text struct {
	body string
}

// Text causes a piece of text to be written in literal form.
func Text(body string) Node {
	return text{
		body: body,
	}
}

// Textf causes a piece of text containing string formatting directives
// to be written after performing substitutions.
func Textf(format string, args ...any) Node {
	return text{
		body: fmt.Sprintf(format, args...),
	}
}

func (n text) writePlainText(w io.StringWriter) (int, error) {
	return w.WriteString(n.body)
}

func (n text) writeVT100(w io.StringWriter, attributesState *vt100AttributesState) (int, error) {
	nTotal, err := attributesState.setCurrentToDesired(w)
	if err != nil {
		return nTotal, err
	}
	nPart, err := w.WriteString(n.body)
	nTotal += nPart
	return nTotal, err
}
