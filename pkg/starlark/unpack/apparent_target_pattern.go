package unpack

import (
	"fmt"

	"github.com/buildbarn/bonanza/pkg/label"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type apparentTargetPatternUnpackerInto struct{}

// ApparentTargetPattern is capable of unpacking string arguments that
// contain an apparent target pattern (e.g., "@rules_swift//...").
//
// If the target pattern is not prefixed with a repo name (e.g.,
// "//...") or relative (e.g., ":all"), the target pattern is resolved
// to be relative to the package to which the current Starlark file
// belongs.
var ApparentTargetPattern UnpackerInto[label.ApparentTargetPattern] = apparentTargetPatternUnpackerInto{}

func (apparentTargetPatternUnpackerInto) UnpackInto(thread *starlark.Thread, v starlark.Value, dst *label.ApparentTargetPattern) error {
	s, ok := starlark.AsString(v)
	if !ok {
		return fmt.Errorf("got %s, want string", v.Type())
	}
	canonicalPackage := label.MustNewCanonicalLabel(thread.CallFrame(1).Pos.Filename()).GetCanonicalPackage()
	tp, err := canonicalPackage.AppendTargetPattern(s)
	if err != nil {
		return fmt.Errorf("invalid target pattern: %w", err)
	}
	*dst = tp
	return nil
}

func (ui apparentTargetPatternUnpackerInto) Canonicalize(thread *starlark.Thread, v starlark.Value) (starlark.Value, error) {
	var tp label.ApparentTargetPattern
	if err := ui.UnpackInto(thread, v, &tp); err != nil {
		return nil, err
	}
	return starlark.String(tp.String()), nil
}

func (apparentTargetPatternUnpackerInto) GetConcatenationOperator() syntax.Token {
	return 0
}
