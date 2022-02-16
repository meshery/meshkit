package manifests

import "github.com/meshery/meshkit/errors"

const (
	ErrGetCrdNamesCode    = "1001"
	ErrGetSchemasCode     = "1002"
	ErrGetAPIVersionCode  = "1003"
	ErrGetAPIGroupCode    = "1004"
	ErrPopulatingYamlCode = "1005"
	ErrAbsentFilterCode   = "1006"
)

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
