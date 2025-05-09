package formatted

import (
	"io"
)

type join struct {
	parts []Node
}

// Join multiple pieces of formatted text together and write them in
// concatenated form.
func Join(parts ...Node) Node {
	return join{
		parts: parts,
	}
}

func (n join) writePlainText(w io.StringWriter) (int, error) {
	var nTotal int
	for _, part := range n.parts {
		nPart, err := part.writePlainText(w)
		nTotal += nPart
		if err != nil {
			return nTotal, err
		}
	}
	return nTotal, nil
}

func (n join) writeVT100(w io.StringWriter, attributesState *vt100AttributesState) (int, error) {
	var nTotal int
	for _, part := range n.parts {
		nPart, err := part.writeVT100(w, attributesState)
		nTotal += nPart
		if err != nil {
			return nTotal, err
		}
	}
	return nTotal, nil
}
