package label

import (
	"errors"
	"regexp"
)

// Module is the name of a Bazel module, as declared by calling module()
// in MODULE.bazel.
type Module struct {
	value string
}

const validModulePattern = "[a-z]([a-z0-9._-]*[a-z0-9])?"

var validModuleRegexp = regexp.MustCompile("^" + validModulePattern + "$")

var errInvalidModule = errors.New("module name must match " + validModulePattern)

// NewModule validates that the provided string is a valid Bazel module
// name. If an instance of Module is returned that wraps the provided
// value.
func NewModule(value string) (Module, error) {
	if !validModuleRegexp.MatchString(value) {
		return Module{}, errInvalidModule
	}
	return Module{value: value}, nil
}

// MustNewModule is the same as NewModule, except that it panics if the
// provided value is not a valid Bazel module name.
func MustNewModule(value string) Module {
	m, err := NewModule(value)
	if err != nil {
		panic(err)
	}
	return m
}

func (m Module) String() string {
	return m.value
}

// ToApparentRepo returns the apparent repo name that should be used if
// the bazel_dep() for this module does not have an explicit repo name
// set. As all module names are also valid apparent repo names, the
// module name is used as is.
func (m Module) ToApparentRepo() ApparentRepo {
	return ApparentRepo{value: m.value}
}

// ToModuleInstance appends a module version to a module name, thereby
// returning a module instance. This can be used to construct unique
// repo names if multiple_version_override() is used.
//
// As multiple_version_override() is only rarely used, this function is
// called without a version number most of the times, causing the
// version number in the module instance to remain empty.
func (m Module) ToModuleInstance(mv *ModuleVersion) ModuleInstance {
	if mv == nil {
		return ModuleInstance{value: m.value + "+"}
	}
	return ModuleInstance{value: m.value + "+" + mv.value}
}
