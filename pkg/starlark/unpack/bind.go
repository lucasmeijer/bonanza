package unpack

import (
	"go.starlark.net/starlark"
)

// UnpackerInto implements a strategy for unpacking a function argument
// of a Starlark function into a Go type.
type UnpackerInto[T any] interface {
	Canonicalizer
	UnpackInto(thread *starlark.Thread, v starlark.Value, dst *T) error
}

type boundUnpacker[T any] struct {
	thread   *starlark.Thread
	dst      *T
	unpacker UnpackerInto[T]
}

// Bind an UnpackerInto to a variable, so that it can unpack a function
// argument and store it at a desired location.
func Bind[T any](thread *starlark.Thread, dst *T, unpacker UnpackerInto[T]) starlark.Unpacker {
	return &boundUnpacker[T]{
		thread:   thread,
		dst:      dst,
		unpacker: unpacker,
	}
}

func (u *boundUnpacker[T]) Unpack(v starlark.Value) error {
	return u.unpacker.UnpackInto(u.thread, v, u.dst)
}
