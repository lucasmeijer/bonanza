package label

import (
	"errors"
	"regexp"
	"strings"

	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
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

func (tn TargetName) String() string {
	return tn.value
}

// GetSibling appends a provided target name to the directory portion of
// the current target name.
func (tn TargetName) GetSibling(child TargetName) TargetName {
	slash := strings.LastIndexByte(tn.value, '/')
	if slash < 0 {
		return child
	}
	return TargetName{value: tn.value[:slash+1] + child.value}
}

// ToComponents returns the pathname components of the target name.
func (tn TargetName) ToComponents() []path.Component {
	v := tn.value
	components := make([]path.Component, 0, strings.Count(v, "/")+1)
	for {
		slash := strings.IndexByte(v, '/')
		if slash < 0 {
			components = append(components, path.MustNewComponent(v))
			return components
		}
		components = append(components, path.MustNewComponent(v[:slash]))
		v = v[slash+1:]
	}
}
