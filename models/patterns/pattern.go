package patterns

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/models/meshmodel/registry"
	regv1beta1 "github.com/meshery/meshkit/models/meshmodel/registry/v1beta1"
	"github.com/meshery/meshkit/utils"
	component "github.com/meshery/schemas/models/v1beta2/component"
	registrycomponent "github.com/meshery/schemas/models/v1beta3/component"
	pattern "github.com/meshery/schemas/models/v1beta3/design"
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
func GetPatternFormat(patternFile string) (*pattern.PatternFile, error) {
	pattern := pattern.PatternFile{}
	err := encoding.Unmarshal([]byte(patternFile), &pattern)
	if err != nil {
		err = utils.ErrDecodeYaml(err)
		return nil, err
	}
	return &pattern, nil
}

func ProcessAnnotations(pattern *pattern.PatternFile) {
	components := []*component.ComponentDefinition{}
	for _, component := range pattern.Components {
		if !component.Metadata.IsAnnotation {
			components = append(components, component)
		}
	}
	pattern.Components = components
}

func ProcessComponentStatus(pattern *pattern.PatternFile) {
	components := []*component.ComponentDefinition{}

	hasComponentStatus := func(component *component.ComponentDefinition) bool {
		return component != nil && component.Status != nil
	}

	for _, component := range pattern.Components {
		if hasComponentStatus(component) && string(*component.Status) == "ignored" {
			continue
		}
		components = append(components, component)
	}
	pattern.Components = components
}

// DehydratePattern removes model and component (schema and capabilities) definitions from the pattern file
func DehydratePattern(pattern *pattern.PatternFile) {
	for _, comp := range pattern.Components {
		comp.Component.Schema = ""
		comp.Capabilities = nil
		if comp.Model != nil {
			comp.ModelReference = comp.Model.ToReference()
		}
		comp.Model = nil
	}
}

// SanitizePattern trims leading and trailing whitespace from every string key
// and string value inside each component's Configuration map, and from each
// component's DisplayName. This prevents gopkg.in/yaml.v3 from quoting map
// keys or values that contain incidental whitespace (e.g. 'storage ': 2Gi)
// when the design is exported as a Kubernetes manifest or Helm chart.
//
// Call SanitizePattern at the design-save boundary, alongside DehydratePattern,
// so that clean data is always persisted to the database.
func SanitizePattern(p *pattern.PatternFile) {
	if p == nil {
		return
	}
	for _, comp := range p.Components {
		if comp == nil {
			continue
		}
		comp.DisplayName = strings.TrimSpace(comp.DisplayName)
		comp.Configuration = sanitizeConfigMap(comp.Configuration)
	}
}

// sanitizeConfigMap recursively trims string keys and string values in a
// map[string]interface{} configuration tree. Non-string leaves (bool, number,
// nil) are passed through unchanged so that schema-typed fields are not
// corrupted.
func sanitizeConfigMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[strings.TrimSpace(k)] = sanitizeConfigValue(v)
	}
	return result
}

func sanitizeConfigValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case map[string]interface{}:
		return sanitizeConfigMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = sanitizeConfigValue(item)
		}
		return result
	default:
		// bool, int64, float64, nil — pass through unchanged.
		return v
	}
}

// HydratePattern adds model and component (schema and capabilities) definitions to the pattern file
func HydratePattern(pattern *pattern.PatternFile, registryManager *registry.RegistryManager) []error {
	entityCache := registry.RegistryEntityCache{}

	errors := []error{}

	for _, comp := range pattern.Components {

		componentFilter := regv1beta1.ComponentFilter{
			Name:       comp.Component.Kind,
			APIVersion: comp.Component.Version,
			ModelName:  comp.ModelReference.Name,
		}

		componentList, _, _, _ := registryManager.GetEntitiesMemoized(&componentFilter, &entityCache)
		if len(componentList) == 0 {
			errors = append(errors, fmt.Errorf("component %s:%s not found in registry", comp.Component.Kind, comp.Component.Version))
			continue // component not found in registry
		}
		// Registry stores v1beta3/component.ComponentDefinition (canonical
		// casing, implements entity.Entity). design.PatternFile.Components is
		// typed v1beta2/component.ComponentDefinition per the schemas type
		// graph (see v1beta3/const.go). The inner Model pointer type
		// (*modelv1beta1.ModelDefinition), Component struct, and Capabilities
		// slice are the same underlying types across both component versions,
		// so these field assignments are safe.
		componentDef := componentList[0].(*registrycomponent.ComponentDefinition)

		comp.Model = componentDef.Model
		comp.Component.Schema = componentDef.Component.Schema
		comp.Capabilities = componentDef.Capabilities
	}
	return errors
}
