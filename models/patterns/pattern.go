package patterns

import (
	"github.com/Masterminds/semver/v3"
	"github.com/layer5io/meshkit/utils"
	"github.com/meshery/schemas/models/patterns"
	"gopkg.in/yaml.v2"
)

func GetNextVersion(p *patterns.PatternFile) (string, error) {
	version, err := semver.NewVersion(p.Version)
	if err != nil {
		return "", err
		// return ErrInvalidVersion(err) // send meshkit error
	}

	nextVersion := version.IncPatch().String()
	return nextVersion, nil
}

func AssignVersion(p *patterns.PatternFile) {
	p.Version = semver.New(0, 0, 1, "", "").String()
}

func GetPatternFormat(patternFile string) (*patterns.PatternFile, error) {
	pattern := patterns.PatternFile{}
	err := yaml.Unmarshal([]byte(patternFile), &pattern)
	if err != nil {
		err = utils.ErrDecodeYaml(err)
		return nil, err
	}
	return &pattern, nil
}
