package component

import "github.com/layer5io/meshkit/errors"

const (
	ErrCrdGenerateCode  = "meshkit-11155"
	ErrDefinitionCode   = "meshkit-11156"
	ErrGetSchemaCode    = "meshkit-11157"
	ErrUpdateSchemaCode = "meshkit-11158"
)

var ErrNoSchemasFound = errors.New(ErrGetSchemaCode, errors.Alert, []string{"Could not get schema for the given openapi spec"}, []string{"The OpenAPI spec doesn't include \"components.schemas\" path"}, []string{"The spec doesn't have include any schema"}, []string{"Verify the spec has valid schema."})

// No reference usage found. Also check in adapters before deleting
func ErrCrdGenerate(err error) error {
	return errors.New(ErrCrdGenerateCode, errors.Alert, []string{"Could not generate component with the given CRD"}, []string{err.Error()}, []string{""}, []string{"Verify CRD has valid schema."})
}

// No reference usage found. Also check in adapters before deleting
func ErrGetDefinition(err error) error {
	return errors.New(ErrDefinitionCode, errors.Alert, []string{"Could not get definition for the given CRD"}, []string{err.Error()}, []string{""}, []string{"Verify CRD has valid schema."})
}

func ErrGetSchema(err error) error {
	return errors.New(ErrGetSchemaCode, errors.Alert, []string{"Could not get schema for the given CRD"}, []string{err.Error()}, []string{"Unable to marshal from cue value to JSON", "Unable to unmarshal from JSON to Go type"}, []string{"Verify CRD has valid schema.", "Malformed JSON provided", "CUE path to propery doesn't exist"})
}

func ErrUpdateSchema(err error, obj string) error {
	return errors.New(ErrUpdateSchemaCode, errors.Alert, []string{"Failed to update schema properties for ", obj}, []string{err.Error()}, []string{"Incorrect type assertion", "Selector.Unquoted might have been invoked on non-string label", "error during conversion from cue.Selector to string"}, []string{"Ensure correct type assertion", "Perform appropriate conversion from cue.Selector to string", "Verify CRD has valid schema"})
}
