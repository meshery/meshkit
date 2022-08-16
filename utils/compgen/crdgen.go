package compgen

import (
	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/utils"
	"github.com/sirupsen/logrus"
)

// should be responsible for one and only one thing
// in this case, it is to generate the components from CRDS
// other conversion stuff should be done beforehand
// it expects the crds to be sent when initialized
type crdComponentGenerator struct {
	crds                []string
	extractorPathConfig CuePathConfig
}

// all paths should be a valid CUE expression
type CuePathConfig struct {
	NamePath       string
	GroupPath      string
	VersionPath    string
	SpecPath       string
	IdentifierPath string // identifiers are the values that uniquely identify a CRD (in most of the cases, it is the 'Name' field)
}

// crds should be yaml
func (cg crdComponentGenerator) generate() ([]Component, error) {
	components := make([]Component, 0)
	for _, crd := range cg.crds {
		// crd shoudl be YAML
		crdCue, err := utils.YamlToCue(crd)
		if err != nil {
			logrus.Warn(ErrCrdYaml(err))
			continue
		}
	}
	return components, nil
}

func NewCrdComponentGenerator(crds []string, pathConf CuePathConfig) ComponentGenerator {
	return &crdComponentGenerator{
		crds:                crds,
		extractorPathConfig: pathConf,
	}
}

func getDefinition(crd cue.Value, pathConf CuePathConfig) (string, error) {
	resourceId, err := utils.Lookup(crd, pathConf.IdentifierPath)
	if err != nil {

	}
	return "", nil
}
