package files

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/layer5io/meshkit/errors"
)

var (
	// Error code
	ErrUnsupportedExtensionCode                    = "meshkit-11282"
	ErrUnsupportedExtensionForOperationCode        = "meshkit-11283"
	ErrFailedToIdentifyFileCode                    = "meshkit-11284"
	ErrSanitizingFileCode                          = "meshkit-11285"
	ErrInvalidYamlCode                             = "meshkit-11286"
	ErrInvalidJsonCode                             = "meshkit-11287"
	ErrFailedToExtractTarCode                      = "meshkit-11288"
	ErrUnsupportedFileTypeCode                     = "meshkit-11289"
	ErrInvalidKubernetesManifestCode               = "meshkit-11290"
	ErrInvalidMesheryDesignCode                    = "meshkit-11291"
	ErrInvalidHelmChartCode                        = "meshkit-11292"
	ErrInvalidDockerComposeCode                    = "meshkit-11293"
	ErrInvalidKustomizationCode                    = "meshkit-11294"
	ErrFileTypeNotSupportedForDesignConversionCode = "meshkit-11300"
)

func ErrUnsupportedExtensionForOperation(operation string, fileName string, fileExt string, supportedExtensions []string) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' has an unsupported extension '%s' for the operation '%s'.", fileName, fileExt, operation),
		fmt.Sprintf("Supported extensions for this operation are: %s.", strings.Join(supportedExtensions, ", ")),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be used for the operation '%s' because the extension '%s' is not supported.", fileName, operation, fileExt),
		fmt.Sprintf("The system is designed to handle files with the following extensions for this operation: %s.", strings.Join(supportedExtensions, ", ")),
	}

	probableCause := []string{
		"The file extension does not match any of the supported formats for this operation.",
		"The file may have been saved with an incorrect or unsupported extension.",
		"The operation may have specific requirements for file types that are not met by this file.",
	}

	remedy := []string{
		"Ensure the file is saved with one of the supported extensions for this operation.",
		"Convert the file to a supported format before attempting the operation.",
		"Check the documentation or requirements for the operation to verify the supported file types.",
	}

	return errors.New(ErrUnsupportedExtensionForOperationCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrUnsupportedExtension(fileName string, fileExt string, supportedExtensionsMap map[string]bool) error {
	supportedExtensions := slices.Collect(maps.Keys(supportedExtensionsMap))

	sdescription := []string{
		fmt.Sprintf("The file '%s' has an unsupported extension: '%s'.", fileName, fileExt),
		fmt.Sprintf("Supported file extensions are: %s.", strings.Join(supportedExtensions, ", ")),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be processed because the extension '%s' is not supported by the system.", fileName, fileExt),
		fmt.Sprintf("The system is designed to handle files with the following extensions: %s.", strings.Join(supportedExtensions, ", ")),
	}

	probableCause := []string{
		"The file extension does not match any of the supported formats.",
		"The file may have been saved with an incorrect or unsupported extension.",
	}

	remedy := []string{
		"Ensure the file is saved with one of the supported extensions.",
		"Convert the file to a supported format before attempting to process it.",
	}

	return errors.New(ErrSanitizingFileCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidYaml(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' contains invalid YAML syntax.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed due to invalid YAML syntax.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The YAML file may contain syntax errors, such as incorrect indentation, missing colons, or invalid characters.",
		"The file may have been corrupted or improperly edited.",
	}

	remedy := []string{
		"Review the YAML syntax in the file and correct any errors.",
		"Use a YAML validator or linter to identify and fix issues.",
		"Ensure the file adheres to the YAML specification.",
	}

	return errors.New(ErrInvalidYamlCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidJson(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' contains invalid JSON syntax.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed due to invalid JSON syntax.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The JSON file may contain syntax errors, such as missing commas, curly braces, or square brackets.",
		"The file may have been corrupted or improperly edited.",
		"Special characters or escape sequences may not have been properly formatted.",
	}

	remedy := []string{
		"Review the JSON syntax in the file and correct any errors.",
		"Use a JSON validator or linter to identify and fix issues.",
		"Ensure the file adheres to the JSON specification (e.g., double quotes for keys and strings).",
		"Check for common issues like trailing commas or unescaped special characters.",
	}

	return errors.New(ErrInvalidJsonCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrFailedToExtractArchive(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("Failed to extract the  archive '%s'.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("An error occurred while attempting to extract the TAR archive '%s'.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The archive may be corrupted or incomplete.",
		"The archive may contain unsupported compression formats or features.",
		"Insufficient permissions to read or write files during extraction.",
		"The  archive may have been created with a different tool or version that is incompatible.",
	}

	remedy := []string{
		"Verify that the  archive is not corrupted by checking its integrity or re-downloading it.",
		"Ensure the archive uses a supported compression format (e.g., .zip, .tar, .tar.gz, ).",
		"Check that you have sufficient permissions to read the archive and write to the destination directory.",
		"Try using a different   extraction tool or library to rule out compatibility issues.",
	}

	return errors.New(ErrFailedToExtractTarCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrFailedToIdentifyFile(fileName string, fileExt string, identificationTrace map[string]error) error {

	validTypes := slices.Collect(maps.Keys(identificationTrace))

	sdescription := []string{
		"The  file '%s' could not be identified as a supported type",
	}

	// Build a detailed trace of identification attempts
	var traceDetails []string
	for fileType, err := range identificationTrace {
		traceDetails = append(traceDetails, fmt.Sprintf("- Attempted to identify as '%s': %v", fileType, err))
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' was not recognized as any of the supported file types %v.", fileName, validTypes),
		"Identification attempts and errors:",
	}
	ldescription = append(ldescription, traceDetails...)

	probableCause := []string{
		"The file extension does not match any of the supported types.",
		"The file may be corrupted or incomplete.",
		"The file may have been saved with an incorrect or unsupported format.",
		"The file may not conform to the expected structure for any supported type.",
	}

	remedy := []string{
		"Ensure the file is saved with one of the supported extensions.",
		"Verify the integrity of the file and ensure it is not corrupted.",
		"Check the file's content and structure to ensure it matches one of the supported types.",
		"Convert the file to a supported format before attempting to process it.",
	}
	return errors.New(ErrUnsupportedFileTypeCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidKubernetesManifest(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' is not a valid Kubernetes manifest.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed as a Kubernetes manifest due to invalid syntax or structure.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The file may contain invalid YAML syntax or incorrect indentation.",
		"The file may not conform to the Kubernetes API schema.",
		"The file may be missing required fields or contain unsupported fields.",
	}

	remedy := []string{
		"Review the YAML syntax in the file and correct any errors.",
		"Use a Kubernetes YAML validator or linter to identify and fix issues.",
		"Ensure the file adheres to the Kubernetes API specification.",
	}

	return errors.New(ErrInvalidKubernetesManifestCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidHelmChart(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' is not a valid Helm chart.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed as a Helm chart due to invalid structure or missing files.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The file may be missing required files such as 'Chart.yaml' or 'values.yaml'.",
		"The file may be corrupted or incomplete.",
		"The file may not conform to the Helm chart specification.",
	}

	remedy := []string{
		"Ensure the file contains all required Helm chart files (e.g., Chart.yaml, values.yaml).",
		"Verify the integrity of the Helm chart archive.",
		"Check the Helm documentation for the correct chart structure.",
	}

	return errors.New(ErrInvalidHelmChartCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidDockerCompose(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' is not a valid Docker Compose file.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed as a Docker Compose file due to invalid syntax or structure.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The file may contain invalid YAML syntax or incorrect indentation.",
		"The file may not conform to the Docker Compose specification.",
		"The file may be missing required fields or contain unsupported fields.",
	}

	remedy := []string{
		"Review the YAML syntax in the file and correct any errors.",
		"Use a Docker Compose validator or linter to identify and fix issues.",
		"Ensure the file adheres to the Docker Compose specification.",
	}

	return errors.New(ErrInvalidDockerComposeCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidKustomization(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' is not a valid Kustomization file.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed as a Kustomization file due to invalid syntax or structure.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The file may be missing required fields such as 'resources' or 'bases'.",
		"The file may contain invalid YAML syntax or incorrect indentation.",
		"The file may not conform to the Kustomize specification.",
	}

	remedy := []string{
		"Review the YAML syntax in the file and correct any errors.",
		"Ensure the file contains all required fields for Kustomization.",
		"Check the Kustomize documentation for the correct file structure.",
	}

	return errors.New(ErrInvalidKustomizationCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrUnsupportedFileTypeForConversionToDesign(fileName string, fileType string) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' of  type '%s' is not supported for  conversion to a design", fileName, fileType),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' of type '%s' cannot be converted to design. Supported formats are: meshery-design, helm-chart, k8s-manifest, docker-compose.", fileName, fileType),
	}

	probableCause := []string{
		"File  doesn't match any supported IAC files",
		"Attempting to convert a file type not enabled for design conversion",
	}

	remedy := []string{
		"Verify the file format matches one of the supported types",
		"Convert the file to a supported format before processing",
	}

	return errors.New(ErrFileTypeNotSupportedForDesignConversionCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

var (
	ErrNoTarInsideOCICode        = "meshkit-11295"
	ErrEmptyOCIImageCode         = "meshkit-11296"
	ErrUnCompressOCIArtifactCode = "meshkit-11297"
	ErrWaklingLocalDirectoryCode = "meshkit-11298"
	ErrDecodePatternCode         = "meshkit-11299"
)

// OCI Parsing errors

func ErrNoTarInsideOCi() error {
	return errors.New(ErrNoTarInsideOCICode, errors.Alert, []string{"No tar file found inside OCI image"}, []string{"Unable to locate the compressed file(.tar.gz) inside the OCI image."}, []string{"The OCI image does not contain a ziped file."}, []string{"Verify that the OCI image contains a ziped file."})
}
func ErrEmptyOCIImage(err error) error {
	return errors.New(ErrEmptyOCIImageCode, errors.Alert, []string{}, []string{}, []string{}, []string{})
}

func ErrUnCompressOCIArtifact(err error) error {
	return errors.New(ErrUnCompressOCIArtifactCode, errors.Alert, []string{"Failed to uncompress OCI artifact"}, []string{err.Error()}, []string{"unable to uncompress OCI artifact", "OCI artifact may be corrupted"}, []string{"check if the OCI artifact is valid and not corrupted"})
}

func ErrWaklingLocalDirectory(err error) error {
	return errors.New(ErrWaklingLocalDirectoryCode, errors.Alert, []string{"Failed to walk local directory"}, []string{err.Error()}, []string{"unable to walk local directory", "local directory may be corrupted"}, []string{"check if the local directory is valid and not corrupted"})
}

func ErrDecodePattern(err error) error {
	return errors.New(ErrDecodePatternCode, errors.Alert, []string{"Error failed to decode design data into go slice"}, []string{err.Error()}, []string{}, []string{})
}

var (
	ErrInvalidModelCode        = "replace_me"
	ErrInvalidModelArchiveCode = "replace_me"
	ErrInvalidModelYamlCode    = "replace_me"
	ErrInvalidModelJsonCode    = "replace_me"
	ErrEmptyModelCode          = "replace_me"
	ErrNoModelFoundCode        = "replace_me"
)

func ErrInvalidModel(filename string, err error) error {
	// return error based on file extension
	fileExt := filepath.Ext(filename)

	isEntityJsonFile := fileExt == ".json"
	isEntityYamlFile := fileExt == ".yaml" || fileExt == ".yml"
	// check prefix as random string is appended to archive during file handling (eg: .tar becomes .tar263831)
	isEntityModel := strings.HasPrefix(fileExt, ".tar") || strings.HasPrefix(fileExt, ".tgz") || strings.HasPrefix(fileExt, ".tar.gz")

	if isEntityJsonFile {
		return ErrInvalidModelJson(filename, err)
	} else if isEntityYamlFile {
		return ErrInvalidModelYaml(filename, err)
	} else if isEntityModel {
		return ErrInvalidModelArchive(filename, err)
	}
	supportedExtensions := []string{".json", ".yaml", ".yml", ".tar", ".tgz", ".tar.gz"}
	return ErrUnsupportedExtensionForOperation("import", filename, fileExt, supportedExtensions)
}

func ErrInvalidModelArchive(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("Failed to extract the archive '%s'.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("An error occurred while attempting to extract the TAR archive '%s'.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The archive may be non OCI compliant.",
		"The archive may be corrupted or incomplete.",
		"The archive may contain unsupported compression formats or features.",
		"Insufficient permissions to read or write files during extraction.",
		"The archive may have been created with a different tool or version that is incompatible.",
	}

	remedy := []string{
		"Make sure the archive is OCI compliant. Meshery Models should be OCI compliant archives.",
		"Verify that the  archive is not corrupted by checking its integrity or re-downloading it.",
		"Ensure the archive uses a supported compression format (e.g., .tar, .tar.gz, ).",
		"Check that you have sufficient permissions to read the archive and write to the destination directory.",
		"Try using a different extraction tool or library to rule out compatibility issues.",
	}

	return errors.New(ErrInvalidModelArchiveCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidModelYaml(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' contains invalid YAML syntax or does not conform to the Meshery model schema.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed due to invalid YAML syntax or does not conform to the Meshery model schema.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The YAML file may contain syntax errors, such as incorrect indentation, missing colons, or invalid characters.",
		"The file may have been corrupted or improperly edited.",
		"Special characters or escape sequences may not have been properly formatted.",
		"The YAML does not conform to the Meshery model schema.",
	}

	remedy := []string{
		"Review the YAML syntax in the file and correct any errors.",
		"Use a YAML validator or linter to identify and fix issues.",
		"Ensure the YAML conforms to the Meshery model schema. You can refer to the Meshery model schema documentation for more details.",
	}

	return errors.New(ErrInvalidModelYamlCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrInvalidModelJson(fileName string, err error) error {
	sdescription := []string{
		fmt.Sprintf("The file '%s' contains invalid JSON syntax or does not conform to the Meshery model schema.", fileName),
	}

	ldescription := []string{
		fmt.Sprintf("The file '%s' could not be parsed due to invalid JSON syntax or does not conform to the Meshery model schema.", fileName),
		fmt.Sprintf("Error details: %s", err.Error()),
	}

	probableCause := []string{
		"The JSON file may contain syntax errors, such as missing commas, curly braces, or square brackets.",
		"The file may have been corrupted or improperly edited.",
		"Special characters or escape sequences may not have been properly formatted.",
		"The JSON does not conform to the Meshery model schema.",
	}

	remedy := []string{
		"Review the JSON syntax in the file and correct any errors.",
		"Use a JSON validator or linter to identify and fix issues.",
		"Ensure the file adheres to the JSON specification (e.g., double quotes for keys and strings).",
		"Check for common issues like trailing commas or unescaped special characters.",
		"Ensure the JSON conforms to the Meshery model schema. You can refer to the Meshery model schema documentation for more details.",
	}

	return errors.New(ErrInvalidModelJsonCode, errors.Critical, sdescription, ldescription, probableCause, remedy)
}

func ErrEmptyModel() error {
	return errors.New(ErrEmptyModelCode, errors.Alert, []string{"No component found in model provided."}, []string{"No component found in model provided. Models must have atleast one component."}, []string{}, []string{})
}
