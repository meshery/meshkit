package manifests

import "github.com/layer5io/meshkit/errors"

const (
	ErrExecutingBinaryCode     = "1000"
	ErrGetCrdNamesCode         = "1001"
	ErrGetSchemasCode          = "1002"
	ErrGetAPIVersionCode       = "1003"
	ErrGetAPIGroupCode         = "1004"
	ErrGetManifestFromHelmCode = "1005"
	ErrParsingJsonCode         = "1006"
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

// func ErrGetManifestFromHelm(err error)error{
// 	return errors.New(ErrGetManifestFromHelmCode,errors.Alert,[]string{""})
// }
func ErrParsingJson(err error) error {
	return errors.New(ErrParsingJsonCode, errors.Alert, []string{"Error parsing json"}, []string{err.Error()}, []string{"Json could not be parsed"}, []string{"Make sure the path passed in filter is correct"})

}
