package compgen

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/sirupsen/logrus"
)

const PrometheusComponentIconURL = "https://google.com"

// all paths should be a valid CUE expression
type CuePathConfig struct {
	NamePath       string
	GroupPath      string
	VersionPath    string
	SpecPath       string
	IdentifierPath string // identifiers are the values that uniquely identify a CRD (in most of the cases, it is the 'Name' field)
}

type prometheusComponentsGenerator struct {
	crds                []string
	extractorPathConfig CuePathConfig
}

func (pcg prometheusComponentsGenerator) generate() ([]v1alpha1.Component, error) {
	components := make([]v1alpha1.Component, 0)
	for _, crd := range pcg.crds {
		meta := map[string]interface{}{
			"icon": PrometheusComponentIconURL,
		}
		comp := v1alpha1.NewComponent()
		// crds should be yaml
		crdCue, err := utils.YamlToCue(crd)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		schema, err := getSchema(crdCue, pcg.extractorPathConfig)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		comp.Spec = schema
		name, err := extractValueFromPath(crdCue, pcg.extractorPathConfig.NamePath)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		meta["name"] = name
		comp.Metadata = meta
	}
	return components, nil
}

func NewPrometheusComponentsGenerator(crds []string, pathConf CuePathConfig) ComponentsGenerator {
	return &prometheusComponentsGenerator{crds: crds, extractorPathConfig: pathConf}
}
