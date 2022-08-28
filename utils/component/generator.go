package component

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/sirupsen/logrus"
)

const DefaultPrometheusComponentIconURL = "https://google.com"

// all paths should be a valid CUE expression
type CuePathConfig struct {
	NamePath    string
	GroupPath   string
	VersionPath string
	SpecPath    string
	// identifiers are the values that uniquely identify a CRD (in most of the cases, it is the 'Name' field)
	IdentifierPath string
}

type ComponentsGenerator struct {
	crds                []string
	extractorPathConfig CuePathConfig
	resourceMetadata    map[string]interface{}
}

func (cg ComponentsGenerator) Generate() ([]v1alpha1.Component, error) {
	components := make([]v1alpha1.Component, 0)
	for _, crd := range cg.crds {
		meta := cg.resourceMetadata
		comp := v1alpha1.NewComponent()
		// crds should be yaml
		crdCue, err := utils.YamlToCue(crd)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}

		schema, err := getSchema(crdCue, cg.extractorPathConfig)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		comp.Spec = schema
		name, err := extractValueFromPath(crdCue, cg.extractorPathConfig.NamePath)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		meta["name"] = name
		// append given metadata
		for k, v := range cg.resourceMetadata {
			meta[k] = v
		}
		comp.Metadata = meta
		components = append(components, comp)
	}
	return components, nil
}

func NewComponentsGenerator(crds []string, pathConf CuePathConfig, meta map[string]interface{}) *ComponentsGenerator {
	return &ComponentsGenerator{crds: crds, extractorPathConfig: pathConf, resourceMetadata: meta}
}
