package component

import (
	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/sirupsen/logrus"
)

const ComponentMetaNameKey = "name"

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
		comp := v1alpha1.NewComponent()
		crdCue, err := utils.YamlToCue(crd)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		// update component spec
		schema, err := getSchema(crdCue, cg.extractorPathConfig)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		comp.Spec = schema
		// update metadata fields
		err = cg.updateComponentMetadata(crdCue, comp)
		if err != nil {
			logrus.Warn(ErrCrdGenerate(err))
			continue
		}
		components = append(components, comp)
	}
	return components, nil
}

func (cg ComponentsGenerator) updateComponentMetadata(crdCue cue.Value, comp v1alpha1.Component) error {
	meta := cg.resourceMetadata
	name, err := extractCueValueFromPath(crdCue, cg.extractorPathConfig.NamePath)
	if err != nil {
		return err
	}
	meta[ComponentMetaNameKey] = name
	for k, v := range cg.resourceMetadata {
		meta[k] = v
	}
	comp.Metadata = meta
	return nil
}

func NewComponentsGenerator(crds []string, pathConf CuePathConfig, meta map[string]interface{}) *ComponentsGenerator {
	return &ComponentsGenerator{crds: crds, extractorPathConfig: pathConf, resourceMetadata: meta}
}
