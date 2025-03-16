package label

import (
	"errors"
	"regexp"
)

// TargetName corresponds to the name of an addressable and/or buildable
// target within a package (e.g., "go_default_library").
type TargetName struct {
	value string
}

var validTargetNameRegexp = regexp.MustCompile("^" + validTargetNamePattern + "$")

var errInvalidTargetName = errors.New("Target name must match " + validTargetNamePattern)

// NewTargetName validates that a string containing a target name is
// valid. If so, an instance of TargetName that wraps the value is
// returned.
func NewTargetName(value string) (TargetName, error) {
	if !validTargetNameRegexp.MatchString(value) {
		return TargetName{}, errInvalidTargetName
	}
	return TargetName{value: value}, nil
}

// MustNewTargetName is the same as NewTargetName, except that it panics
// if the provided value is not a valid target name.
func MustNewTargetName(value string) TargetName {
	identifier, err := NewTargetName(value)
	if err != nil {
		panic(err)
	}
	return identifier
}

func (tn TargetName) String() string {
	return tn.value
}
