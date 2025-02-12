package manifests

import "github.com/layer5io/meshkit/errors"

const (
	ErrGetCrdNamesCode           = "meshkit-11233"
	ErrGetSchemasCode            = "meshkit-11234"
	ErrGetAPIVersionCode         = "meshkit-11235"
	ErrGetAPIGroupCode           = "meshkit-11236"
	ErrPopulatingYamlCode        = "meshkit-11237"
	ErrAbsentFilterCode          = "meshkit-11238"
	ErrCreatingDirectoryCode     = "meshkit-11239"
	ErrGetResourceIdentifierCode = "meshkit-11240"
)

func ErrGetResourceIdentifier(err error) error {
	return errors.New(ErrGetResourceIdentifierCode, errors.Alert, 
		[]string{"Error extracting the resource identifier name."}, 
		[]string{err.Error()}, 
		[]string{"Could not extract the value with the given filter configuration."}, 
		[]string{
			"Make sure to input a valid manifest.",
			"Make sure to provide the right filter configurations.",
			"Make sure the filters are appropriate for the given manifest.",
		})
}

func ErrGetCrdNames(err error) error {
	return errors.New(ErrGetCrdNamesCode, errors.Alert, 
		[]string{"Error getting CRD names."}, 
		[]string{err.Error()}, 
		[]string{"Could not execute kubeopenapi-jsonschema correctly."}, 
		[]string{
			"Make sure the binary is valid and correct.",
			"Make sure the filter passed is correct.",
		})
}

func ErrGetSchemas(err error) error {
	return errors.New(ErrGetSchemasCode, errors.Alert, 
		[]string{"Error getting schemas."}, 
		[]string{err.Error()}, 
		[]string{"Schemas JSON could not be produced from given CRD."}, 
		[]string{"Make sure the filter passed is correct."})
}

func ErrGetAPIVersion(err error) error {
	return errors.New(ErrGetAPIVersionCode, errors.Alert, 
		[]string{"Error getting API version."}, 
		[]string{err.Error()}, 
		[]string{"API version could not be parsed."}, 
		[]string{"Make sure the filter passed is correct."})
}

func ErrGetAPIGroup(err error) error {
	return errors.New(ErrGetAPIGroupCode, errors.Alert, 
		[]string{"Error getting API group."}, 
		[]string{err.Error()}, 
		[]string{"API group could not be parsed."}, 
		[]string{"Make sure the filter passed is correct."})
}

func ErrPopulatingYaml(err error) error {
	return errors.New(ErrPopulatingYamlCode, errors.Alert, 
		[]string{"Error populating YAML."}, 
		[]string{err.Error()}, 
		[]string{"YAML could not be populated with the returned manifests."}, 
		[]string{"Please verify the manifest format and try again."})
}

func ErrAbsentFilter(err error) error {
	return errors.New(ErrAbsentFilterCode, errors.Alert, 
		[]string{"Error with passed filters."}, 
		[]string{err.Error()}, 
		[]string{"ItrFilter or ItrSpecFilter is either not passed or empty."}, 
		[]string{"Please pass the correct ItrFilter and ItrSpecFilter."})
}

func ErrCreatingDirectory(err error) error {
	return errors.New(ErrCreatingDirectoryCode, errors.Alert, 
		[]string{"Could not create directory."}, 
		[]string{err.Error()}, 
		[]string{"Proper file permissions were not set."}, 
		[]string{"Please check the appropriate file permissions."})
}
