package patterns

import (
	"github.com/Masterminds/semver/v3"
	"github.com/meshery/schemas/models/v1beta1/pattern"
)

func GetNextVersion(p *pattern.PatternFile) (string, error) {
	// Existing patterns do not have version hence when trying to assign next version for such patterns, it will fail the validation.
	// Hence, if version is not present, start versioning for those afresh.
	if p.Version == "" {
		AssignVersion(p)
		return p.Version, nil
	}
	version, err := semver.NewVersion(p.Version)
	if err != nil {
		return "", err
		// return ErrInvalidVersion(err) // send meshkit error
	}

	nextVersion := version.IncPatch().String()
	return nextVersion, nil
}

func AssignVersion(p *pattern.PatternFile) {
	p.Version = semver.New(0, 0, 1, "", "").String()
}
