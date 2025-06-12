package label

import (
	"errors"
	"regexp"
	"strings"
)

// CanonicalRepo corresponds to the canonical name of a repo, without
// the leading "@@". Canonical repo names can refer to a module instance
// (e.g., "rules_go+") or a repo declared by a module extension or repo
// rule (e.g., "gazelle++go_deps+org_golang_x_lint").
type CanonicalRepo struct {
	value string
}

const validCanonicalRepoPattern = validModuleInstancePattern +
	`(\+` + validStarlarkIdentifierPattern + `\+` + validApparentRepoPattern + `)?`

var validCanonicalRepoRegexp = regexp.MustCompile("^" + validCanonicalRepoPattern + "$")

var errInvalidCanonicalRepo = errors.New("canonical repo must match " + validCanonicalRepoPattern)

// NewCanonicalRepo validates that the provided string is a canonical
// repo name. If so, an instance of CanonicalRepo is returned that wraps
// its value.
func NewCanonicalRepo(value string) (CanonicalRepo, error) {
	if !validCanonicalRepoRegexp.MatchString(value) {
		return CanonicalRepo{}, errInvalidCanonicalRepo
	}
	return CanonicalRepo{value: value}, nil
}

func (r CanonicalRepo) String() string {
	return r.value
}

// GetModuleInstance strips the name of a module extension and apparent
// repo, if any, from the canonical repo name.
func (r CanonicalRepo) GetModuleInstance() ModuleInstance {
	versionIndex := strings.IndexByte(r.value, '+') + 1
	if offset := strings.IndexByte(r.value[versionIndex:], '+'); offset >= 0 {
		return ModuleInstance{value: r.value[:versionIndex+offset]}
	}
	return ModuleInstance{value: r.value}
}

// GetRootPackage returns the canonical package corresponding to
// top-level package in this repo. This can be used to construct labels
// for files that are typically placed in the root directory of a repo
// (e.g., MODULE.bazel, REPO.bazel).
func (r CanonicalRepo) GetRootPackage() CanonicalPackage {
	return CanonicalPackage{value: "@@" + r.value}
}

// GetModuleExtension returns true if the canonical repo belongs to a
// repo that was declared by a module extension. If so, the name of the
// module extension and apparent repo are returned separately.
func (r CanonicalRepo) GetModuleExtension() (ModuleExtension, ApparentRepo, bool) {
	repoIndex := strings.LastIndexByte(r.value, '+')
	moduleExtension := r.value[:repoIndex]
	if strings.IndexByte(moduleExtension, '+') < 0 {
		return ModuleExtension{}, ApparentRepo{}, false
	}
	return ModuleExtension{value: moduleExtension},
		ApparentRepo{value: r.value[repoIndex+1:]},
		true
}

func (r CanonicalRepo) applyToLabelOrTargetPattern(value string) string {
	if offset := strings.IndexByte(value, '/'); offset > 0 {
		// Translate "@from//x/y:z" to "@@to//x/y:z".
		return "@@" + r.value + value[offset:]
	}
	// Translate "@from" to "@@to//:from".
	return "@@" + r.value + "//:" + strings.TrimLeft(value, "@")
}
