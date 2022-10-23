package component

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
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

var DefaultPathConfig = CuePathConfig{
	NamePath:       "spec.names.kind",
	IdentifierPath: "spec.names.kind",
	VersionPath:    "spec.versions[0].name",
	GroupPath:      "spec.group",
	SpecPath:       "spec.versions[0].schema.openAPIV3Schema.properties.spec",
}

var DefaultPathConfig2 = CuePathConfig{
	NamePath:       "spec.names.kind",
	IdentifierPath: "spec.names.kind",
	VersionPath:    "spec.versions[0].name",
	GroupPath:      "spec.group",
	SpecPath:       "spec.validation.openAPIV3Schema.properties.spec",
}

var Configs = []CuePathConfig{DefaultPathConfig, DefaultPathConfig2}

func Generate(crd string) (v1alpha1.Component, error) {
	component := v1alpha1.NewComponent()
	crdCue, err := utils.YamlToCue(crd)
	if err != nil {
		return component, err
	}
	var schema string
	for _, cfg := range Configs {
		schema, err = getSchema(crdCue, cfg)
		if err == nil {
			break
		}
	}
	component.Spec = schema
	name, err := extractCueValueFromPath(crdCue, DefaultPathConfig.NamePath)
	if err != nil {
		return component, err
	}
	metadata := map[string]interface{}{}
	metadata[ComponentMetaNameKey] = name
	component.Metadata = metadata
	return component, nil
}
