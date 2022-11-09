package component

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/manifests"
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

func Generate(crd string) (v1alpha1.ComponentDefinition, error) {
	component := v1alpha1.ComponentDefinition{}
	component.Metadata.Metadata = make(map[string]interface{})
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
	component.Schema = schema
	name, err := extractCueValueFromPath(crdCue, DefaultPathConfig.NamePath)
	if err != nil {
		return component, err
	}
	version, err := extractCueValueFromPath(crdCue, DefaultPathConfig.VersionPath)
	if err != nil {
		return component, err
	}
	component.Kind = name
	component.APIVersion = version
	component.Format = v1alpha1.JSON
	component.Metadata.Metadata["display-name"] = manifests.FormatToReadableString(name)
	return component, nil
}
