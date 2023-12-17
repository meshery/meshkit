package registry

import (
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

type EntityErrorCount struct {
	Attempt int
	Error   error
}

type EntityCountWithErrors struct {
	Model        map[string]EntityErrorCount
	Registry     map[string]EntityErrorCount
	Component    map[string]EntityErrorCount
	Relationship map[uuid.UUID]EntityErrorCount
	Policy       map[uuid.UUID]EntityErrorCount
}

var RegisterAttempts EntityCountWithErrors

var NonImportModel map[string]v1alpha1.EntitySummary

func init() {
	RegisterAttempts = EntityCountWithErrors{
		Model:        make(map[string]EntityErrorCount),
		Registry:     make(map[string]EntityErrorCount),
		Component:    make(map[string]EntityErrorCount),
		Relationship: make(map[uuid.UUID]EntityErrorCount),
		Policy:       make(map[uuid.UUID]EntityErrorCount),
	}
	NonImportModel = make(map[string]v1alpha1.EntitySummary)
}

var (
	ErrUnknownHostCode = "11097"
	ErrEmptySchemaCode = "11098"
)

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, []string{"Registrant type is not supported or unknown."}, []string{err.Error()}, []string{"The host registering a Model and it's components is not recognized by Meshery Server (or by the version currently running)."}, []string{"Validate the name and location of the model registrant. Try upgrading to latest available Meshery version."})
}
func ErrEmptySchema() error {
	return errors.New(ErrEmptySchemaCode, errors.Alert, []string{"Empty schema for the component"}, nil, []string{"The schema is empty for the component."}, []string{"For the particular component the schema is empty. Use the docs or discussion forum for more details  "})
}

func onModelError(reg Registry, modelName string, h Host, err error) {
	if entityCount, ok := RegisterAttempts.Model[modelName]; ok {
		entityCount.Attempt++
		entityCount.Error = err
		RegisterAttempts.Model[modelName] = entityCount
	} else {
		RegisterAttempts.Model[modelName] = EntityErrorCount{Attempt: 1, Error: err}
	}

	if RegisterAttempts.Model[modelName].Attempt == 1 {
		currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
		currentValue.Models++
		NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
	}
}

func onRegistrantError(h Host) {
	if entityCount, ok := RegisterAttempts.Registry[HostnameToPascalCase(h.Hostname)]; ok {
		entityCount.Attempt++
		RegisterAttempts.Registry[HostnameToPascalCase(h.Hostname)] = entityCount
	} else {
		RegisterAttempts.Registry[HostnameToPascalCase(h.Hostname)] = EntityErrorCount{Attempt: 1}
	}
}

func onEntityError(en Entity, h Host, err error) {
	switch entity := en.(type) {
	case v1alpha1.ComponentDefinition:
		entityName := entity.DisplayName
		if err == nil {
			err = ErrEmptySchema()
		}
		if entityCount, ok := RegisterAttempts.Component[entityName]; ok {
			entityCount.Attempt++

			entityCount.Error = err
			RegisterAttempts.Component[entityName] = entityCount
		} else {
			RegisterAttempts.Component[entityName] = EntityErrorCount{Attempt: 1, Error: err}
		}

		if RegisterAttempts.Component[entityName].Attempt == 1 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Components++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}

	case v1alpha1.RelationshipDefinition:
		entityID := entity.ID
		if entityCount, ok := RegisterAttempts.Relationship[entityID]; ok {
			entityCount.Attempt++
			entityCount.Error = err
			RegisterAttempts.Relationship[entityID] = entityCount
		} else {
			RegisterAttempts.Relationship[entityID] = EntityErrorCount{Attempt: 1, Error: err}
		}

		if RegisterAttempts.Relationship[entityID].Attempt == 1 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Relationships++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}

	case v1alpha1.PolicyDefinition:
		entityID := entity.ID
		if entityCount, ok := RegisterAttempts.Policy[entityID]; ok {
			entityCount.Attempt++
			entityCount.Error = err
			RegisterAttempts.Policy[entityID] = entityCount
		} else {
			RegisterAttempts.Policy[entityID] = EntityErrorCount{Attempt: 1, Error: err}
		}

		if RegisterAttempts.Policy[entityID].Attempt == 1 {
			currentValue := NonImportModel[HostnameToPascalCase(h.Hostname)]
			currentValue.Policies++
			NonImportModel[HostnameToPascalCase(h.Hostname)] = currentValue
		}
	}
}
