package component

import "github.com/layer5io/meshkit/errors"

const (
	ErrCrdGenerateCode  = "meshkit-11155"
	ErrDefinitionCode   = "meshkit-11156"
	ErrGetSchemaCode    = "meshkit-11157"
	ErrUpdateSchemaCode = "meshkit-11158"
)

var ErrNoSchemasFound = errors.New(ErrGetSchemaCode, errors.Alert, 
	[]string{"No schemas found in the OpenAPI specification."}, 
	[]string{"The OpenAPI specification does not include 'components.schemas' path."}, 
	[]string{"The specification is missing required schema definitions."}, 
	[]string{"Please ensure the OpenAPI specification includes valid schema definitions."})

// No reference usage found. Also check in adapters before deleting
func ErrCrdGenerate(err error) error {
	return errors.New(ErrCrdGenerateCode, errors.Alert, 
		[]string{"Failed to generate component from the CRD."}, 
		[]string{err.Error()}, 
		[]string{"The CRD schema is invalid or malformed."}, 
		[]string{"Please verify that the CRD contains a valid schema definition."})
}

// No reference usage found. Also check in adapters before deleting
func ErrGetDefinition(err error) error {
	return errors.New(ErrDefinitionCode, errors.Alert, 
		[]string{"Failed to retrieve definition from the CRD."}, 
		[]string{err.Error()}, 
		[]string{"The CRD definition is invalid or missing required fields."}, 
		[]string{"Please ensure the CRD contains all required definition fields."})
}

func ErrGetSchema(err error) error {
	return errors.New(ErrGetSchemaCode, errors.Alert, 
		[]string{"Failed to retrieve schema from the CRD."}, 
		[]string{err.Error()}, 
		[]string{
			"Unable to convert CUE value to JSON format.",
			"Unable to parse JSON into Go type.",
		}, 
		[]string{
			"Please verify the CRD contains a valid schema.",
			"Check for proper JSON formatting.",
			"Ensure the CUE path to property exists.",
		})
}

func ErrUpdateSchema(err error, obj string) error {
	return errors.New(ErrUpdateSchemaCode, errors.Alert, 
		[]string{"Failed to update schema properties for " + obj + "."}, 
		[]string{err.Error()}, 
		[]string{
			"Type assertion failed during schema update.",
			"Invalid conversion from CUE.Selector to string.",
			"Non-string label used with Selector.Unquoted.",
		}, 
		[]string{
			"Please verify type assertions are correct.",
			"Ensure proper conversion from CUE.Selector to string.",
			"Check that the CRD schema is valid.",
		})
}
