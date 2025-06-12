package label

import (
	"errors"
	"regexp"
)

// StarlarkIdentifier corresponds to an identifier of a Starlark
// function, variable or constant.
type StarlarkIdentifier struct {
	value string
}

const (
	validStarlarkIdentifierPattern = `[a-zA-Z_]\w*`
)

var validStarlarkIdentifierRegexp = regexp.MustCompile("^" + validStarlarkIdentifierPattern + "$")

var errInvalidStarlarkIdentifier = errors.New("Starlark identifier must match " + validStarlarkIdentifierPattern)

// NewStarlarkIdentifier validates that a string containing a Starlark
// identifier is valid. If so, an instance of StarlarkIdentifier that
// wraps the value is returned.
func NewStarlarkIdentifier(value string) (StarlarkIdentifier, error) {
	if !validStarlarkIdentifierRegexp.MatchString(value) {
		return StarlarkIdentifier{}, errInvalidStarlarkIdentifier
	}
	return StarlarkIdentifier{value: value}, nil
}

func (i StarlarkIdentifier) String() string {
	return i.value
}

// IsPublic returns true if the Starlark identifier refers to a
// function, variable or constant that is public (i.e., not beginning
// with an underscore).
func (i StarlarkIdentifier) IsPublic() bool {
	return i.value[0] != '_'
}
