package schemaProvider

import (
	"fmt"
	"os"
)

func getSchemaMap() map[string]string {
	return map[string]string{
		"application": "./configuration/applicationImport.json",
		"filter":      "./configuration/filterImport.json",
		"design":      "./configuration/designImport.json",
	}
}

func getUiSchemaMap() map[string]string {
	return map[string]string{
		"application": "./configuration/uiSchemaApplication.json",
		"design":      "./configuration/uiSchemaDesignImport.json",
	}
}

// ServeJSonFile serves the content of the JSON schema along with the uiSchema if any is present
func ServeJSonFile(resourceName string) ([]byte, []byte, error) {
	schemaLocation := getSchemaMap()[resourceName]
	if schemaLocation == "" {
		return nil, nil, fmt.Errorf("requested resource's (%s) schema is not found", resourceName)
	}

	jsonContent, err := os.ReadFile(schemaLocation)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading json file: %s", err)
	}

	uiSchemaLocation := getUiSchemaMap()[resourceName]
	if uiSchemaLocation == "" {
		return jsonContent, nil, nil
	}

	uiSchemaJsonContent, err := os.ReadFile(uiSchemaLocation)
	if err != nil {
		return jsonContent, nil, nil
	}

	return jsonContent, uiSchemaJsonContent, nil
}
