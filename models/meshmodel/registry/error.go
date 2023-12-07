package registry

import (
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

type EntityCount struct {
	Model        map[string]int
	Registry     map[string]int
	Component    map[string]int
	Relationship map[uuid.UUID]int
	Policy       map[uuid.UUID]int
}

var RegisterAttempts EntityCount

var NonImportModel map[string]v1alpha1.EntitySummary

func init() {
	RegisterAttempts = EntityCount{
		Model:        make(map[string]int),
		Registry:     make(map[string]int),
		Component:    make(map[string]int),
		Relationship: make(map[uuid.UUID]int),
		Policy:       make(map[uuid.UUID]int),
	}
	NonImportModel = make(map[string]v1alpha1.EntitySummary)
}

var (
	ErrUnknownHostCode = "11097"
)

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, []string{"Registrant type is not supported or unknown."}, []string{err.Error()}, []string{"The host registering a Model and it's components is not recognized by Meshery Server (or by the version currently running)."}, []string{"Validate the name and location of the model registrant. Try upgrading to latest available Meshery version."})
}
func onModelError(reg Registry, modelName string, h Host) {
	if RegisterAttempts.Model[modelName] == 0 {
		currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
		currentValue.Models++
		NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
	}
	RegisterAttempts.Model[modelName]++
}

func onRegistrantError(h Host) {
	RegisterAttempts.Registry[HostnameToPascalCase(h.Hostname)]++
}
func onEntityError(en Entity, h Host) {

	switch entity := en.(type) {
	case v1alpha1.ComponentDefinition:
		if RegisterAttempts.Component[entity.DisplayName] == 0 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Components++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}
		RegisterAttempts.Component[entity.DisplayName]++

	case v1alpha1.RelationshipDefinition:
		if RegisterAttempts.Relationship[entity.ID] == 0 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Relationships++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}
		RegisterAttempts.Relationship[entity.ID]++

	case v1alpha1.PolicyDefinition:
		if RegisterAttempts.Policy[entity.ID] == 0 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Policies++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}
		RegisterAttempts.Policy[entity.ID]++

	}

}
