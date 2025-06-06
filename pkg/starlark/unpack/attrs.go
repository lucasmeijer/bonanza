package unpack

import (
	"errors"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// AttrUnpacker specifies how individual named attributes of a
// struct-like object need to be unpacked.
type AttrUnpacker struct {
	Name     string
	Unpacker starlark.Unpacker
}

type attrsUnpackerInto[T any] struct {
	nestedUnpackersCreator func(thread *starlark.Thread, dst *T) []AttrUnpacker
}

// Attrs can be used to unpack struct-like objects that have named
// attributes into native structs. Whenever unpacking is performed, a
// callback is invoked that is responsible for yielding new unpackers
// for each of the named attributes that is requested.
func Attrs[T any](nestedUnpackersCreator func(thread *starlark.Thread, dst *T) []AttrUnpacker) UnpackerInto[T] {
	return &attrsUnpackerInto[T]{
		nestedUnpackersCreator: nestedUnpackersCreator,
	}
}

func (ui *attrsUnpackerInto[T]) UnpackInto(thread *starlark.Thread, v starlark.Value, dst *T) error {
	hasAttrs, ok := v.(starlark.HasAttrs)
	if !ok {
		return errors.New("value does not have attributes")
	}

	nestedUnpackers := ui.nestedUnpackersCreator(thread, dst)
	for _, nestedUnpacker := range nestedUnpackers {
		nestedV, err := hasAttrs.Attr(thread, nestedUnpacker.Name)
		if err != nil {
			return fmt.Errorf("attribute %#v: %w", nestedUnpacker.Name, err)
		}
		if nestedV == nil {
			return fmt.Errorf("attribute %#v does not exist", nestedUnpacker.Name)
		}
		if err := nestedUnpacker.Unpacker.Unpack(nestedV); err != nil {
			return fmt.Errorf("attribute %#v: %w", nestedUnpacker.Name, err)
		}
	}
	return nil
}

func (attrsUnpackerInto[T]) Canonicalize(thread *starlark.Thread, v starlark.Value) (starlark.Value, error) {
	// The best we could do is create some replacement struct(), but
	// because this unpacker also works on non-struct objects this
	// may not be desirable.
	return v, nil
}

func (attrsUnpackerInto[T]) GetConcatenationOperator() syntax.Token {
	return 0
}
