package label

import (
	"errors"
	"regexp"
	"strings"
)

// ModuleInstance is a pair of a module name and an optional module
// version. It is the leading part of a canonical repo name.
type ModuleInstance struct {
	value string
}

const validModuleInstancePattern = validModulePattern + `\+` + `(` + canonicalModuleVersionPattern + `)?`

var validModuleInstanceRegexp = regexp.MustCompile("^" + validModuleInstancePattern + "$")

var errInvalidModuleInstance = errors.New("module instance must match " + validModuleInstancePattern)

// NewModuleInstance validates that the provided string value
// corresponds to a valid Bazel module instance (i.e., module name and
// optional module version). Upon success, a ModuleInstance is returned
// that wraps the provided value.
func NewModuleInstance(value string) (ModuleInstance, error) {
	if !validModuleInstanceRegexp.MatchString(value) {
		return ModuleInstance{}, errInvalidModuleInstance
	}
	return ModuleInstance{value: value}, nil
}

// MustNewModuleInstance is the same as NewModuleInstance, except that
// it panics if the provided module instance is invalid.
func MustNewModuleInstance(value string) ModuleInstance {
	mi, err := NewModuleInstance(value)
	if err != nil {
		panic(err)
	}
	return mi
}

func (mi ModuleInstance) String() string {
	return mi.value
}

// GetModule returns the leading module name that is part of the module
// instance.
func (mi ModuleInstance) GetModule() Module {
	return Module{value: mi.value[:strings.IndexByte(mi.value, '+')]}
}

// GetModuleVersion returns the trailing module version number that is
// part of the module instance. As module version numbers are optional,
// this function also returns a boolean value to indicate presence of
// the version number.
func (mi ModuleInstance) GetModuleVersion() (ModuleVersion, bool) {
	version := mi.value[strings.IndexByte(mi.value, '+')+1:]
	if version == "" {
		return ModuleVersion{}, false
	}
	return ModuleVersion{value: version}, true
}

// GetBareCanonicalRepo converts a module instance to a canonical repo
// name. This can be used to refer to the repo corresponding to the
// module instance itself (not one of its repos declared using module
// extensions or repo rules).
func (mi ModuleInstance) GetBareCanonicalRepo() CanonicalRepo {
	return CanonicalRepo{value: mi.value}
}

// GetModuleExtension appends a Starlark identifier to the name of a
// module instance. This can be used to uniquely identify a module
// extension.
func (mi ModuleInstance) GetModuleExtension(extensionName StarlarkIdentifier) ModuleExtension {
	return ModuleExtension{value: mi.value + `+` + extensionName.value}
}
