package manifests

import "github.com/layer5io/meshkit/errors"

const (
	ErrGetCrdNamesCode           = "1001"
	ErrGetSchemasCode            = "1002"
	ErrGetAPIVersionCode         = "1003"
	ErrGetAPIGroupCode           = "1004"
	ErrPopulatingYamlCode        = "1005"
	ErrAbsentFilterCode          = "1006"
	ErrCreatingDirectoryCode     = "1007"
	ErrGetResourceIdentifierCode = "11075"
)

func ErrGetResourceIdentifier(err error) error {
	return errors.New(ErrGetResourceIdentifierCode, errors.Alert, []string{"Error extracting the resource identifier name"}, []string{err.Error()}, []string{"Could not extract the value with the given filter configuration"}, []string{"Make sure to input a valid manifest", "Make sure to provide the right filter configurations", "Make sure the filters are appropriate for the given manifest"})
}

func ErrGetCrdNames(err error) error {
	return errors.New(ErrGetCrdNamesCode, errors.Alert, []string{"Error getting crd names"}, []string{err.Error()}, []string{"Could not execute kubeopenapi-jsonschema correctly"}, []string{"Make sure the binary is valid and correct", "Make sure the filter passed is correct"})
}

func ErrGetSchemas(err error) error {
	return errors.New(ErrGetSchemasCode, errors.Alert, []string{"Error getting schemas"}, []string{err.Error()}, []string{"Schemas Json could not be produced from given crd."}, []string{"Make sure the filter passed is correct"})
}
func ErrGetAPIVersion(err error) error {
	return errors.New(ErrGetAPIVersionCode, errors.Alert, []string{"Error getting api version"}, []string{err.Error()}, []string{"Api version could not be parsed"}, []string{"Make sure the filter passed is correct"})
}
func ErrGetAPIGroup(err error) error {
	return errors.New(ErrGetAPIGroupCode, errors.Alert, []string{"Error getting api group"}, []string{err.Error()}, []string{"Api group could not be parsed"}, []string{"Make sure the filter passed is correct"})
}

func ErrPopulatingYaml(err error) error {
	return errors.New(ErrPopulatingYamlCode, errors.Alert, []string{"Error populating yaml"}, []string{err.Error()}, []string{"Yaml could not be populated with the returned manifests"}, []string{""})
}
func ErrAbsentFilter(err error) error {
	return errors.New(ErrAbsentFilterCode, errors.Alert, []string{"Error with passed filters"}, []string{err.Error()}, []string{"ItrFilter or ItrSpecFilter is either not passed or empty"}, []string{"Pass the correct ItrFilter and ItrSpecFilter"})
}
func ErrCreatingDirectory(err error) error {
	return errors.New(ErrCreatingDirectoryCode, errors.Alert, []string{"could not create directory"}, []string{err.Error()}, []string{"proper file permissions were not set"}, []string{"check the appropriate file permissions"})
}
