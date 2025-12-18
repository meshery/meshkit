package patterns

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/models/meshmodel/registry"
	regv1beta1 "github.com/meshery/meshkit/models/meshmodel/registry/v1beta1"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas/models/v1beta1/component"
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
		componentDef := componentList[0].(*component.ComponentDefinition)

		comp.Model = componentDef.Model
		comp.Component.Schema = componentDef.Component.Schema
		comp.Capabilities = componentDef.Capabilities
	}
	return errors
}
