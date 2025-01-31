package files

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/layer5io/meshkit/errors"
)

var (
	// Error code
	ErrUnsupportedExtensionCode = "replace_me"
	ErrFailedToIdentifyFileCode = "replace_me"
	ErrSanitizingFileCode       = "replace_me"
	ErrInvalidYamlCode          = "replace_me"
	ErrInvalidJsonCode          = "replace_me"
	ErrFailedToExtractTarCode   = "replace_me"
)

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
