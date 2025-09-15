package utils

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/meshery/meshkit/errors"
)

var (
	ErrUnmarshalCode                 = "meshkit-11159"
	ErrUnmarshalInvalidCode          = "meshkit-11160"
	ErrUnmarshalSyntaxCode           = "meshkit-11161"
	ErrUnmarshalTypeCode             = "meshkit-11162"
	ErrUnmarshalUnsupportedTypeCode  = "meshkit-11163"
	ErrUnmarshalUnsupportedValueCode = "meshkit-11164"
	ErrMarshalCode                   = "meshkit-11165"
	ErrGetBoolCode                   = "meshkit-11166"
	ErrInvalidProtocolCode           = "meshkit-11167"
	ErrRemoteFileNotFoundCode        = "meshkit-11168"
	ErrReadingRemoteFileCode         = "meshkit-11169"
	ErrReadingLocalFileCode          = "meshkit-11170"
	ErrReadFileCode                  = "meshkit-11171"
	ErrWriteFileCode                 = "meshkit-11172"
	ErrGettingLatestReleaseTagCode   = "meshkit-11173"
	ErrInvalidProtocol               = errors.New(ErrInvalidProtocolCode, errors.Alert, []string{"invalid protocol: only http, https and file are valid protocols"}, []string{}, []string{"Network protocol is incorrect"}, []string{"Make sure to specify the right network protocol"})
	ErrMissingFieldCode              = "meshkit-11174"
	ErrExpectedTypeMismatchCode      = "meshkit-11175"
	ErrJsonToCueCode                 = "meshkit-11176"
	ErrYamlToCueCode                 = "meshkit-11177"
	ErrJsonSchemaToCueCode           = "meshkit-11178"
	ErrCueLookupCode                 = "meshkit-11179"
	ErrTypeCastCode                  = "meshkit-11180"
	ErrCreateFileCode                = "meshkit-11181"
	ErrCreateDirCode                 = "meshkit-11182"
	// ErrDecodeYamlCode represents the error which is generated when yaml
	// decode process fails
	ErrDecodeYamlCode               = "meshkit-11183"
	ErrExtractTarXZCode             = "meshkit-11184"
	ErrExtractZipCode               = "meshkit-11185"
	ErrReadDirCode                  = "meshkit-11186"
	ErrInvalidSchemaVersionCode     = "meshkit-11273"
	ErrFileWalkDirCode              = "meshkit-11274"
	ErrRelPathCode                  = "meshkit-11275"
	ErrCopyFileCode                 = "meshkit-11276"
	ErrCloseFileCode                = "meshkit-11277"
	ErrCompressToTarGZCode          = "meshkit-11248"
	ErrUnsupportedTarHeaderTypeCode = "meshkit-11282"

	ErrConvertToByteCode = "meshkit-11187"

	ErrOpenFileCode = "meshkit-11278"

	// Google Sheets Service Errors
	ErrGoogleJwtInvalidCode = "meshkit-11279"
	ErrGoogleSheetSRVCode   = "meshkit-11280"

	ErrWritingIntoFileCode = "meshkit-11281"
)

func ErrInvalidConstructSchemaVersion(contruct string, version string, supportedVersion string) error {

	return errors.New(
		ErrInvalidSchemaVersionCode,
		errors.Critical,
		[]string{"Invalid schema version " + version},
		[]string{"The schemaVersion key is either empty or has an incorrect value."},
		[]string{fmt.Sprintf("The schema is not of type '%s'", contruct)},
		[]string{"Verify that schemaVersion key should be '%s'", supportedVersion},
	)
}

var (
	ErrExtractType = errors.New(
		ErrUnmarshalTypeCode,
		errors.Alert,
		[]string{"Invalid extraction type"},
		[]string{"The file type to be extracted is neither tar.gz nor zip."},
		[]string{"Invalid object format. The file is not of type zip or tar.gz."},
		[]string{"Make sure to check that the file type is zip or tar.gz."},
	)
	ErrInvalidSchemaVersion = errors.New(
		ErrInvalidSchemaVersionCode,
		errors.Alert,
		[]string{"Invalid schema version"},
		[]string{"The schemaVersion key is either empty or has an incorrect value."},
		[]string{"The schema is not of type 'relationship', 'component', 'model' , 'policy'."},
		[]string{"Verify that schemaVersion key should be either relationships.meshery.io, component.meshery.io, model.meshery.io or policy.meshery.io."},
	)
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

func ErrConvertToByte(err error) error {
	return errors.New(ErrConvertToByteCode, errors.Alert, []string{("error converting data to []byte")}, []string{err.Error()}, []string{"Unsupported data types", "invalid configuration data", "failed serialization of data"}, []string{"check for any custom types in the data that might not be serializable", "Verify that the data type being passed is valid for conversion to []byte"})
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

func ErrTypeCast(err error) error {
	return errors.New(ErrTypeCastCode, errors.Alert, []string{"invaid type assertion requested"}, []string{err.Error()}, []string{"The interface type is not compatible with the request type cast"}, []string{"use correct data type for type casting"})
}

// ErrDecodeYaml is the error when the yaml unmarshal fails
func ErrDecodeYaml(err error) error {
	return errors.New(ErrDecodeYamlCode, errors.Alert, []string{"Error occurred while decoding YAML"}, []string{err.Error()}, []string{}, []string{})
}

// ErrCompressTar is the error for zipping a file into targz
func ErrCompressToTarGZ(err error, path string) error {
	return errors.New(ErrCompressToTarGZCode, errors.Alert, []string{fmt.Sprintf("Error while compressing file %s", path)}, []string{err.Error()}, []string{"The file might be corrupt", "Insufficient permissions to read the file"}, []string{"Verify sufficient read permissions"})
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

func ErrFileWalkDir(err error, path string) error {
	return errors.New(
		ErrFileWalkDirCode,
		errors.Alert,
		[]string{"Error while walking through directory"},
		[]string{err.Error()},
		[]string{fmt.Sprintf("The directory %s does not exist.", path)},
		[]string{"Verify that the correct directory path is provided."},
	)
}
func ErrRelPath(err error, path string) error {
	return errors.New(
		ErrRelPathCode,
		errors.Alert,
		[]string{"Error determining relative path"},
		[]string{err.Error()},
		[]string{("The provided directory path is incorrect."), "The user might not have sufficient permission."},
		[]string{"Verify the provided directory path is correct and if the user has sufficent permission."},
	)
}
func ErrCopyFile(err error) error {
	return errors.New(
		ErrCopyFileCode,
		errors.Alert,
		[]string{"Error copying file"},
		[]string{err.Error()},
		[]string{("The file might not be accessible or the source and destination files are the same."), "The file might be corrupted."},
		[]string{("Ensure the source and destination files are accessible and try again."), "Verify the integrity of the file and try again."},
	)
}

func ErrCloseFile(err error) error {
	return errors.New(
		ErrCloseFileCode,
		errors.Alert,
		[]string{"Error closing file"},
		[]string{err.Error()},
		[]string{("Disk space might be full or the file might be corrupted."), "The user might not have sufficient permission."},
		[]string{"Check for issues with file permissions or disk space and try again."},
	)
}
func ErrOpenFile(file string) error {
	return errors.New(ErrOpenFileCode, errors.Alert, []string{"unable to open file: ", file}, []string{}, []string{"The file does not exist in the location"}, []string{"Make sure to upload the correct file"})
}

func ErrGoogleJwtInvalid(err error) error {
	return errors.New(ErrGoogleJwtInvalidCode, errors.Alert, []string{"Invalid JWT credentials"}, []string{err.Error()}, []string{"Invalid JWT credentials"}, []string{"Make sure to provide valid JWT credentials"})
}

func ErrGoogleSheetSRV(err error) error {
	return errors.New(ErrGoogleSheetSRVCode, errors.Alert, []string{"Error while creating Google Sheets Service"}, []string{err.Error()}, []string{"Issue happened with Google Sheets Service"}, []string{"Make you provide valid JWT credentials and Spreadsheet ID"})
}

func ErrWritingIntoFile(err error, obj string) error {
	return errors.New(ErrWritingIntoFileCode, errors.Alert, []string{fmt.Sprintf("failed to write into file %s", obj)}, []string{err.Error()}, []string{"Insufficient permissions to write into file", "file might be corrupted"}, []string{"check if sufficient permissions are givent to the file", "check if the file is corrupted"})
}

func ErrUnsupportedTarHeaderType(typeflag byte, name string) error {
	return errors.New(
		ErrUnsupportedTarHeaderTypeCode,
		errors.Alert,
		[]string{"Unsupported tar header type encountered during extraction"},
		[]string{fmt.Sprintf("The tar archive contains an entry '%s' with an unsupported type flag '%c'.", name, typeflag)},
		[]string{"The tar archive is malformed or contains an entry type that this utility cannot handle (e.g., special device files)."},
		[]string{"Ensure the tar archive was created with standard file types (directories, regular files, symlinks).", "Check the integrity of the archive file."},
	)
}
