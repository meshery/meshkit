package component

import (
	"encoding/json"
	"errors"
	"fmt"

	"cuelang.org/go/cue"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/sirupsen/logrus"
)

const ComponentMetaNameKey = "name"

// all paths should be a valid CUE expression
type CuePathConfig struct {
	NamePath       string
	GroupPath      string
	VersionPath    string
	SpecPath       string
	ScopePath      string
	PropertiesPath string
	// identifiers are the values that uniquely identify a CRD (in most of the cases, it is the 'Name' field)
	IdentifierPath string
}

var DefaultPathConfig = CuePathConfig{
	NamePath:       "spec.names.kind",
	IdentifierPath: "spec.names.kind",
	VersionPath:    "spec.versions[0].name",
	GroupPath:      "spec.group",
	ScopePath:      "spec.scope",
	SpecPath:       "spec.versions[0].schema.openAPIV3Schema",
	PropertiesPath: "properties",
}

var DefaultPathConfig2 = CuePathConfig{
	NamePath:       "spec.names.kind",
	IdentifierPath: "spec.names.kind",
	VersionPath:    "spec.versions[0].name",
	GroupPath:      "spec.group",
	ScopePath:      "spec.scope",
	SpecPath:       "spec.validation.openAPIV3Schema",
}

var OpenAPISpecPathConfig = CuePathConfig{
	NamePath:       `x-kubernetes-group-version-kind"[0].kind`,
	IdentifierPath: "spec.names.kind",
	VersionPath:    `"x-kubernetes-group-version-kind"[0].version`,
	GroupPath:      `"x-kubernetes-group-version-kind"[0].group`,
	ScopePath:      "spec.scope",
	SpecPath:       "spec.versions[0].schema.openAPIV3Schema",
	PropertiesPath: "properties",
}

var Configs = []CuePathConfig{DefaultPathConfig, DefaultPathConfig2}

func IncludeComponentBasedOnGroup(resource string, groupFilter string) (bool, error) {
	if groupFilter == "" {
		return true, nil
	}

	crdCue, err := utils.YamlToCue(resource)

	if err != nil {
		return false, err
	}

	group, err := extractCueValueFromPath(crdCue, DefaultPathConfig.GroupPath)

	if err != nil {
		logrus.Infof("Failed to extract group from CRD: %v", err)
	}

	return group == groupFilter, nil

}

func Generate(resource string) (component.ComponentDefinition, error) {
	cmp := component.ComponentDefinition{}
	cmp.SchemaVersion = v1beta1.ComponentSchemaVersion

	cmp.Metadata = component.ComponentDefinition_Metadata{}
	crdCue, err := utils.YamlToCue(resource)
	if err != nil {
		return cmp, err
	}

	var schema string
	for _, cfg := range Configs {
		schema, err = getSchema(crdCue, cfg)
		if err == nil {
			break
		}
	}
	cmp.Component.Schema = schema
	name, err := extractCueValueFromPath(crdCue, DefaultPathConfig.NamePath)
	if err != nil {
		return cmp, err
	}
	version, err := extractCueValueFromPath(crdCue, DefaultPathConfig.VersionPath)
	if err != nil {
		return cmp, err
	}
	group, err := extractCueValueFromPath(crdCue, DefaultPathConfig.GroupPath)

	if err != nil {
		return cmp, err
	}
	// return component, err Ignore error if scope isn't found
	if cmp.Metadata.AdditionalProperties == nil {
		cmp.Metadata.AdditionalProperties = make(map[string]interface{})
	}
	scope, _ := extractCueValueFromPath(crdCue, DefaultPathConfig.ScopePath)
	if scope == "Cluster" {
		cmp.Metadata.IsNamespaced = false
	} else if scope == "Namespaced" {
		cmp.Metadata.IsNamespaced = true
	}
	cmp.Component.Kind = name
	if group != "" {
		cmp.Component.Version = fmt.Sprintf("%s/%s", group, version)
	} else {
		cmp.Component.Version = version
	}

	cmp.Format = component.JSON
	cmp.DisplayName = manifests.FormatToReadableString(name)
	return cmp, nil
}

/*
Find and modify specific schema properties.
1. Identify interesting properties by walking entire schema.
2. Store path to interesting properties. Finish walk.
3. Iterate all paths and modify properties.
5. If error occurs, return nil and skip modifications.
*/
func UpdateProperties(fieldVal cue.Value, cuePath cue.Path, group string) (map[string]interface{}, error) {
	rootPath := fieldVal.Path().Selectors()

	compProperties := fieldVal.LookupPath(cuePath)
	crd, err := fieldVal.MarshalJSON()
	if err != nil {
		return nil, ErrUpdateSchema(err, group)
	}

	modified := make(map[string]interface{})
	pathSelectors := [][]cue.Selector{}

	err = json.Unmarshal(crd, &modified)
	if err != nil {
		return nil, ErrUpdateSchema(err, group)
	}

	compProperties.Walk(func(c cue.Value) bool {
		return true
	}, func(c cue.Value) {
		val := c.LookupPath(cue.ParsePath(`"x-kubernetes-preserve-unknown-fields"`))
		if val.Exists() {
			child := val.Path().Selectors()
			childM := child[len(rootPath):(len(child) - 1)]
			pathSelectors = append(pathSelectors, childM)
		}
	})

	// "pathSelectors" contains all the paths from root to the property which needs to be modified.
	for _, selectors := range pathSelectors {
		var m interface{}
		m = modified
		index := 0

		for index < len(selectors) {
			selector := selectors[index]
			selectorType := selector.Type()
			s := selector.String()
			if selectorType == cue.IndexLabel {
				t, ok := m.([]interface{})
				if !ok {
					return nil, ErrUpdateSchema(errors.New("error converting to []interface{}"), group)
				}
				token := selector.Index()
				m, ok = t[token].(map[string]interface{})
				if !ok {
					return nil, ErrUpdateSchema(errors.New("error converting to map[string]interface{}"), group)
				}
			} else {
				if selectorType == cue.StringLabel {
					s = selector.Unquoted()
				}
				t, ok := m.(map[string]interface{})
				if !ok {
					return nil, ErrUpdateSchema(errors.New("error converting to map[string]interface{}"), group)
				}
				m = t[s]
			}
			index++
		}

		t, ok := m.(map[string]interface{})
		if !ok {
			return nil, ErrUpdateSchema(errors.New("error converting to map[string]interface{}"), group)
		}
		delete(t, "x-kubernetes-preserve-unknown-fields")
		if m == nil {
			m = make(map[string]interface{})
		}
		t["type"] = "string"
		t["format"] = "textarea"
	}
	return modified, nil
}
