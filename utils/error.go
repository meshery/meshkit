package utils

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrUnmarshalCode                 = "11043"
	ErrUnmarshalInvalidCode          = "11044"
	ErrUnmarshalSyntaxCode           = "11045"
	ErrUnmarshalTypeCode             = "11046"
	ErrUnmarshalUnsupportedTypeCode  = "11047"
	ErrUnmarshalUnsupportedValueCode = "11048"
	ErrMarshalCode                   = "11049"
	ErrGetBoolCode                   = "11050"
	ErrInvalidProtocolCode           = "11051"
	ErrRemoteFileNotFoundCode        = "11052"
	ErrReadingRemoteFileCode         = "11053"
	ErrReadingLocalFileCode          = "11054"
	ErrReadFileCode                  = "11106"
	ErrWriteFileCode                 = "11110"
	ErrGettingLatestReleaseTagCode   = "11055"
	ErrInvalidProtocol               = errors.New(ErrInvalidProtocolCode, errors.Alert, []string{"invalid protocol: only http, https and file are valid protocols"}, []string{}, []string{"Network protocol is incorrect"}, []string{"Make sure to specify the right network protocol"})
	ErrMissingFieldCode              = "11076"
	ErrExpectedTypeMismatchCode      = "11079"
	ErrJsonToCueCode                 = "11085"
	ErrYamlToCueCode                 = "11086"
	ErrJsonSchemaToCueCode           = "11087"
	ErrCueLookupCode                 = "11089"
	ErrTypeCastCode                  = "11100"
	ErrCreateFileCode                = "11111"
	ErrCreateDirCode                 = "11117"
	// ErrDecodeYamlCode represents the error which is generated when yaml
	// decode process fails
	ErrDecodeYamlCode   = "11035"
	ErrExtractTarXZCode = "11112"
	ErrExtractZipCode   = "11113"
	ErrReadDirCode      = "11114"
)

func ErrCueLookup(err error) error {
	return errors.New(ErrCueLookupCode, errors.Alert, []string{"Could not lookup the given path in the CUE value"}, []string{err.Error()}, []string{""}, []string{"make sure that the path is a valid cue expression and is correct", "make sure that there exists a field with the given path", "make sure that the given root value is correct"})
}

func ErrJsonSchemaToCue(err error) error {
	return errors.New(ErrJsonSchemaToCueCode, errors.Alert, []string{"Could not convert given JsonSchema into a CUE Value"}, []string{err.Error()}, []string{"Invalid jsonschema"}, []string{"Make sure that the given value is a valid JSONSCHEMA"})
}

func ErrYamlToCue(err error) error {
	return errors.New(ErrYamlToCueCode, errors.Alert, []string{"Could not convert given yaml object into a CUE Value"}, []string{err.Error()}, []string{"Invalid yaml"}, []string{"Make sure that the given value is a valid YAML"})
}

func ErrJsonToCue(err error) error {
	return errors.New(ErrJsonToCueCode, errors.Alert, []string{"Could not convert given json object into a CUE Value"}, []string{err.Error()}, []string{"Invalid json object"}, []string{"Make sure that the given value is a valid JSON"})
}

func ErrExpectedTypeMismatch(err error, expectedType string) error {
	return errors.New(ErrExpectedTypeMismatchCode, errors.Alert, []string{"Expected the type to be: ", expectedType}, []string{err.Error()}, []string{"Invalid manifest"}, []string{"Make sure that the value provided in the manifest has the needed type."})
}

func ErrMissingField(err error, missingFieldName string) error {
	return errors.New(ErrMissingFieldCode, errors.Alert, []string{"Missing field or property with name: ", missingFieldName}, []string{err.Error()}, []string{"Invalid manifest"}, []string{"Make sure that the concerned data type has all the required fields/values."})
}

func ErrUnmarshal(err error) error {
	return errors.New(ErrUnmarshalCode, errors.Alert, []string{"Unmarshal unknown error: "}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalInvalid(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalInvalidCode, errors.Alert, []string{"Unmarshal invalid error for type: ", typ.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalSyntax(err error, offset int64) error {
	return errors.New(ErrUnmarshalSyntaxCode, errors.Alert, []string{"Unmarshal syntax error at offest: ", strconv.Itoa(int(offset))}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalType(err error, value string) error {
	return errors.New(ErrUnmarshalTypeCode, errors.Alert, []string{"Unmarshal type error at key: %s. Error: %s", value}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalUnsupportedType(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalUnsupportedTypeCode, errors.Alert, []string{"Unmarshal unsupported type error at key: ", typ.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalUnsupportedValue(err error, value reflect.Value) error {
	return errors.New(ErrUnmarshalUnsupportedValueCode, errors.Alert, []string{"Unmarshal unsupported value error at key: ", value.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrMarshal(err error) error {
	return errors.New(ErrMarshalCode, errors.Alert, []string{"Marshal error, Description: %s"}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrGetBool(key string, err error) error {
	return errors.New(ErrGetBoolCode, errors.Alert, []string{"Error while getting Boolean value for key: %s, error: %s", key}, []string{err.Error()}, []string{"Not a valid boolean"}, []string{"Make sure it is a boolean"})
}

func ErrRemoteFileNotFound(url string) error {
	return errors.New(ErrRemoteFileNotFoundCode, errors.Alert, []string{"remote file not found at", url}, []string{}, []string{"File doesnt exist in the location", "File name is incorrect"}, []string{"Make sure to input the right file name and location"})
}

func ErrReadingRemoteFile(err error) error {
	return errors.New(ErrReadingRemoteFileCode, errors.Alert, []string{"error reading remote file"}, []string{err.Error()}, []string{"File doesnt exist in the location", "File name is incorrect"}, []string{"Make sure to input the right file name and location"})
}

func ErrReadingLocalFile(err error) error {
	return errors.New(ErrReadingLocalFileCode, errors.Alert, []string{"error reading local file"}, []string{err.Error()}, []string{"File does not exist in the location (~/.kube/config)", "File is absent. Filename is not 'config'.", "Insufficient permissions to read file"}, []string{"Verify that the available kubeconfig is accessible by Meshery Server - verify sufficient file permissions (only needs read permission)."})
}

func ErrReadFile(err error, filepath string) error {
	return errors.New(ErrReadFileCode, errors.Alert, []string{"error reading file"}, []string{err.Error()}, []string{fmt.Sprintf("File does not exist in the location %s", filepath), "Insufficient permissions"}, []string{"Verify that file exist at the provided location", "Verify sufficient file permissions."})
}

func ErrWriteFile(err error, filepath string) error {
	return errors.New(ErrWriteFileCode, errors.Alert, []string{"error writing file"}, []string{err.Error()}, []string{fmt.Sprintf("File does not exist in the location %s", filepath), "Insufficient write permissions"}, []string{"Verify that file exist at the provided location", "Verify sufficient file permissions."})
}

func ErrCreateFile(err error, filepath string) error {
	return errors.New(ErrCreateFileCode, errors.Alert, []string{fmt.Sprintf("error creating file at %s", filepath)}, []string{err.Error()}, []string{"invalid path provided", "insufficient permissions"}, []string{"provide a valid path", "retry by using an absolute path", "check for sufficient permissions for the user"})
}

func ErrCreateDir(err error, filepath string) error {
	return errors.New(ErrCreateDirCode, errors.Alert, []string{fmt.Sprintf("error creating directory at %s", filepath)}, []string{err.Error()}, []string{"invalid path provided", "insufficient permissions"}, []string{"provide a valid path", "retry by using an absolute path", "check for sufficient permissions for the user"})
}

func ErrGettingLatestReleaseTag(err error) error {
	return errors.New(
		ErrGettingLatestReleaseTagCode,
		errors.Alert,
		[]string{"Could not fetch latest stable release from github"},
		[]string{err.Error()},
		[]string{"Failed to make GET request to github", "Invalid response received on github.com/<org>/<repo>/releases/stable"},
		[]string{"Make sure Github is reachable", "Make sure a valid response is available on github.com/<org>/<repo>/releases/stable"},
	)
}

func ErrTypeCast(valType string) error {
	return errors.New(ErrTypeCastCode, errors.Alert, []string{"invaid type assertion requested"}, []string{fmt.Sprintf("The underlying type of the interface is %s", valType)}, []string{"The interface type is not compatible with the request type cast"}, []string{"use correct data type for type casting"})
}

// ErrDecodeYaml is the error when the yaml unmarshal fails
func ErrDecodeYaml(err error) error {
	return errors.New(ErrDecodeYamlCode, errors.Alert, []string{"Error occurred while decoding YAML"}, []string{err.Error()}, []string{}, []string{})
}

// ErrExtractTarXVZ is the error for unzipping the targz file
func ErrExtractTarXZ(err error, path string) error {
	return errors.New(ErrExtractTarXZCode, errors.Alert, []string{fmt.Sprintf("Error while extracting file at %s", path)}, []string{err.Error()}, []string{"The gzip might be corrupt"}, []string{})
}

// ErrExtractZip is the error for unzipping the zip file
func ErrExtractZip(err error, path string) error {
	return errors.New(ErrExtractZipCode, errors.Alert, []string{fmt.Sprintf("Error while extracting file at %s", path)}, []string{err.Error()}, []string{"The zip might be corrupt"}, []string{})
}

func ErrReadDir(err error, dirPath string) error {
	return errors.New(ErrReadDirCode, errors.Alert, []string{"error reading directory"}, []string{err.Error()}, []string{fmt.Sprintf("Directory does not exist at the location %s", dirPath), "Insufficient permissions"}, []string{"Verify that directory exist at the provided location", "Verify sufficient directory read permission."})
}
