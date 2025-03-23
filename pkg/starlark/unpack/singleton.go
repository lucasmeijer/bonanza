package unpack

import (
	"go.starlark.net/starlark"
)

type singletonUnpackerInto[T any] struct {
	UnpackerInto[T]
}

// Singleton is capable of unpacking a value and placing it in a slice
// containing a single element. This unpacker is typically used in
// combination with Or() in functions having arguments that can either
// be scalar or a list (e.g., for backward compatibility).
func Singleton[T any](base UnpackerInto[T]) UnpackerInto[[]T] {
	return &singletonUnpackerInto[T]{
		UnpackerInto: base,
	}
}

func (ui *singletonUnpackerInto[T]) UnpackInto(thread *starlark.Thread, v starlark.Value, dst *[]T) error {
	var instance [1]T
	if err := ui.UnpackerInto.UnpackInto(thread, v, &instance[0]); err != nil {
		return err
	}
	*dst = instance[:]
	return nil
}

func (ui *singletonUnpackerInto[T]) Canonicalize(thread *starlark.Thread, v starlark.Value) (starlark.Value, error) {
	element, err := ui.UnpackerInto.Canonicalize(thread, v)
	if err != nil {
		return nil, err
	}
	return starlark.NewList([]starlark.Value{element}), nil
}
