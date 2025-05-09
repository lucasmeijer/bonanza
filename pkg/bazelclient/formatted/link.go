package formatted

import (
	"io"
)

type link struct {
	url  string
	base Node
}

// Link causes the emitted text to be a clickable link. This is only
// supported in terminals that support "OSC 8" style hyperlinks.
func Link(url string, base Node) Node {
	return &link{
		url:  url,
		base: base,
	}
}

func (n *link) writePlainText(w io.StringWriter) (int, error) {
	return n.base.writePlainText(w)
}

func (n *link) writeVT100(w io.StringWriter, attributesState *vt100AttributesState) (int, error) {
	// Start of link.
	nTotal, err := w.WriteString("\x1b]8;;")
	if err != nil {
		return nTotal, err
	}

	nPart, err := w.WriteString(n.url)
	nTotal += nPart
	if err != nil {
		return nTotal, err
	}

	nPart, err = w.WriteString("\x1b\\")
	nTotal += nPart
	if err != nil {
		return nTotal, err
	}

	// Clickable text.
	nPart, err = n.base.writeVT100(w, attributesState)
	nTotal += nPart
	if err != nil {
		return nTotal, err
	}

	// End of link.
	nPart, err = w.WriteString("\x1b]8;;\x1b\\")
	nTotal += nPart
	return nTotal, err
}
