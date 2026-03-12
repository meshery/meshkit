package schema

import (
	"fmt"
	"strings"

	schemav1alpha3 "github.com/meshery/schemas/models/v1alpha3"
	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
)

const (
	relationshipSchemaLocation = "schemas/constructs/v1alpha3/relationship/relationship.yaml#/components/schemas/RelationshipDefinition"
	designSchemaLocation       = "schemas/constructs/v1beta1/design/design.yaml"
	modelSchemaLocation        = "schemas/constructs/v1beta1/model/model.yaml"
	componentSchemaLocation    = "schemas/constructs/v1beta1/component/component.yaml"
	connectionSchemaLocation   = "schemas/constructs/v1beta1/connection/connection.yaml"
	environmentSchemaLocation  = "schemas/constructs/v1beta1/environment/environment.yaml"
)

const (
	modelSchemaVersion        = schemav1beta1.ModelSchemaVersion
	designSchemaVersion       = schemav1beta1.DesignSchemaVersion
	componentSchemaVersion    = schemav1beta1.ComponentSchemaVersion
	connectionSchemaVersion   = schemav1beta1.ConnectionSchemaVersion
	relationshipSchemaVersion = schemav1alpha3.RelationshipSchemaVersion
)

func builtinRegistrations() []Registration {
	return []Registration{
		{
			Ref: Ref{
				SchemaVersion: modelSchemaVersion,
				Type:          TypeModel,
			},
			Location: modelSchemaLocation,
		},
		{
			Ref: Ref{
				SchemaVersion: componentSchemaVersion,
				Type:          TypeComponent,
			},
			Location: componentSchemaLocation,
		},
		{
			Ref: Ref{
				SchemaVersion: connectionSchemaVersion,
				Type:          TypeConnection,
			},
			Location: connectionSchemaLocation,
		},
		{
			Ref: Ref{
				SchemaVersion: designSchemaVersion,
				Type:          TypeDesign,
			},
			Location: designSchemaLocation,
		},
		{
			Ref: Ref{
				SchemaVersion: relationshipSchemaVersion,
				Type:          TypeRelationship,
			},
			Location: relationshipSchemaLocation,
		},
		{
			// Environment schema has no SchemaVersion intentionally: the upstream
			// schemas module does not yet define an EnvironmentSchemaVersion constant.
			// As a result, auto-detection via Validate(data) is not supported for
			// environment documents; callers must use ValidateBytes(Ref{Type: TypeEnvironment}, data)
			// or ValidateAs(TypeEnvironment, data) explicitly.
			Ref: Ref{
				Type: TypeEnvironment,
			},
			Location: environmentSchemaLocation,
		},
	}
}

// Register adds or replaces a schema registration.
func (v *Validator) Register(registration Registration) error {
	if registration.Ref.IsZero() {
		return ErrInvalidRegistration(fmt.Errorf("schema ref is empty"))
	}

	registration.Location = strings.TrimSpace(registration.Location)
	if registration.Location == "" {
		return ErrInvalidRegistration(fmt.Errorf("schema location is empty"))
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if registration.Ref.SchemaVersion != "" {
		v.registrations[schemaVersionKey(registration.Ref.SchemaVersion)] = registration
	}

	// Type key uses "latest wins" semantics: if multiple schema versions are registered
	// for the same type, the most recently registered one becomes the default for
	// type-only lookups (e.g. ValidateAs). This is intentional for the current single-version
	// use case; if multi-version support is needed in the future, this should be revisited.
	if registration.Ref.Type != "" {
		v.registrations[typeKey(registration.Ref.Type)] = registration
	}

	return nil
}

func (v *Validator) resolve(ref Ref) (Registration, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if ref.SchemaVersion != "" {
		if registration, ok := v.registrations[schemaVersionKey(ref.SchemaVersion)]; ok {
			return registration, nil
		}
		// SchemaVersion was explicitly specified but has no registered schema.
		// Do NOT fall back to the Type key – that would silently validate the document
		// against a different schema than the caller requested.
		return Registration{}, ErrResolveSchema(ref)
	}

	if ref.Type != "" {
		if registration, ok := v.registrations[typeKey(ref.Type)]; ok {
			return registration, nil
		}
	}

	return Registration{}, ErrResolveSchema(ref)
}

func schemaVersionKey(schemaVersion string) string {
	return "schemaVersion:" + schemaVersion
}

func typeKey(documentType DocumentType) string {
	return "type:" + string(documentType)
}
