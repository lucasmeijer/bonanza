package label

import (
	"errors"
	"regexp"
)

// ApparentRepo can store the name of an apparent repo, such as
// "bazel_tools" or "rules_go".
type ApparentRepo struct {
	value string
}

const validApparentRepoPattern = `([a-zA-Z][-.\w]*|_builtins)`

var validApparentRepoRegexp = regexp.MustCompile("^" + validApparentRepoPattern + "$")

var errInvalidApparentRepo = errors.New("apparent repo name must match " + validApparentRepoPattern)

// NewApparentRepo validates that the provided string is an apparent
// repo name. Upon success, an instance of ApparentRepo is returned that
// wraps the value.
func NewApparentRepo(value string) (ApparentRepo, error) {
	if !validApparentRepoRegexp.MatchString(value) {
		return ApparentRepo{}, errInvalidApparentRepo
	}
	return ApparentRepo{value: value}, nil
}

// MustNewApparentRepo is the same as NewApparentRepo, except that it
// panics if the provided string is not an apparent repo name.
func MustNewApparentRepo(value string) ApparentRepo {
	r, err := NewApparentRepo(value)
	if err != nil {
		panic(err)
	}
	return r
}

func (r ApparentRepo) String() string {
	return r.value
}
