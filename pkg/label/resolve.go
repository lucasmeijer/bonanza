package label

import (
	"fmt"
)

// Resolver is called into by Resolve() and Canonicalize() to determine
// the canonical repo name that should be prepended to an apparent label
// or target pattern in order to make it canonical.
type Resolver interface {
	GetCanonicalRepo(fromCanonicalRepo CanonicalRepo, toApparentRepo ApparentRepo) (*CanonicalRepo, error)
	GetRootModule() (Module, error)
}

// Resolve an apparent label, so that any leading apparent repo or "@@"
// is replaced with its canonical counterpart. If the apparent repo does
// not exist, the resolved label contains an error message.
func Resolve(resolver Resolver, fromRepo CanonicalRepo, toApparent ApparentLabel) (ResolvedLabel, error) {
	if toCanonical, ok := toApparent.AsCanonical(); ok {
		// Label was already canonical. Nothing to do.
		return toCanonical.AsResolved(), nil
	}

	if toApparentRepo, ok := toApparent.GetApparentRepo(); ok {
		// Label is prefixed with an apparent repo. Resolve the repo.
		toCanonicalRepo, err := resolver.GetCanonicalRepo(fromRepo, toApparentRepo)
		if err != nil {
			return ResolvedLabel{}, fmt.Errorf("failed to resolve apparent repo %#v: %w", toApparentRepo.String(), err)
		}
		if toCanonicalRepo == nil {
			// Repo does not exist. Still permit the
			// label to be constructed, so that
			// analysis of other targets may pass.
			return toApparent.AsResolvedWithError(
				fmt.Sprintf(
					"unknown repo '%s' requested from %s",
					toApparentRepo.String(),
					fromRepo.String(),
				),
			), nil
		}
		return toApparent.WithCanonicalRepo(*toCanonicalRepo).AsResolved(), nil
	}

	// Label is prefixed with "@@". Resolve to the root module.
	rootModule, err := resolver.GetRootModule()
	if err != nil {
		return ResolvedLabel{}, fmt.Errorf("failed to resolve root module: %w", err)
	}
	return toApparent.WithCanonicalRepo(rootModule.ToModuleInstance(nil).GetBareCanonicalRepo()).AsResolved(), nil
}

// Canonicalizable may be implemented by any apparent type (e.g.,
// apparent label, apparent target pattern) that has a canonical
// counterpart. It is used by Canonicalize() to convert an apparent
// label or target pattern to a canonical instance.
type Canonicalizable[T any] interface {
	AsCanonical() (T, bool)
	GetApparentRepo() (ApparentRepo, bool)
	WithCanonicalRepo(canonicalRepo CanonicalRepo) T
}

// Canonicalize an apparent label or target pattern, so that any leading
// apparent repo or "@@" is replaced with its canonical counterpart.
// This function is identical to Resolve(), except that it fails if the
// apparent repo contained in the label or target pattern is unknown.
func Canonicalize[TCanonical any, TApparent Canonicalizable[TCanonical]](resolver Resolver, fromRepo CanonicalRepo, toApparent TApparent) (TCanonical, error) {
	if toCanonical, ok := toApparent.AsCanonical(); ok {
		// Label was already canonical. Nothing to do.
		return toCanonical, nil
	}

	if toApparentRepo, ok := toApparent.GetApparentRepo(); ok {
		// Label is prefixed with an apparent repo. Resolve the repo.
		toCanonicalRepo, err := resolver.GetCanonicalRepo(fromRepo, toApparentRepo)
		if err != nil {
			var bad TCanonical
			return bad, fmt.Errorf("failed to resolve apparent repo %#v: %w", toApparentRepo.String(), err)
		}
		if toCanonicalRepo == nil {
			var bad TCanonical
			return bad, fmt.Errorf("unknown apparent repo @%s referenced by repo @@%s", toApparentRepo.String(), fromRepo.String())
		}
		return toApparent.WithCanonicalRepo(*toCanonicalRepo), nil
	}

	// Label is prefixed with "@@". Resolve to the root module.
	rootModule, err := resolver.GetRootModule()
	if err != nil {
		var bad TCanonical
		return bad, fmt.Errorf("failed to resolve root module: %w", err)
	}
	return toApparent.WithCanonicalRepo(rootModule.ToModuleInstance(nil).GetBareCanonicalRepo()), nil
}
