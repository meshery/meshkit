package schemas

import (
	"fmt"
)

func getSchemaMap() map[string]string {
	return map[string]string{
		"application": "configuration/applicationImport.json",
		"filter":      "configuration/filterImport.json",
		"design":      "configuration/designImport.json",
		"publish":     "configuration/publishCatalogItem.json",
		"helmRepo":    "connections/helmConnection/helmRepoConnection.json",
		"environment": "configuration/environment.json",
	}
}

func getUiSchemaMap() map[string]string {
	return map[string]string{
		"application": "configuration/uiSchemaApplication.json",
		"design":      "configuration/uiSchemaDesignImport.json",
		"filter":      "configuration/uiSchemaFilter.json",
		"publish":     "configuration/uiSchemaPublishCatalogItem.json",
		"helmRepo":    "connections/helmConnection/uiHelmRepoConnection.json",
		"environment": "configuration/uiSchemaEnvironment.json",
	}
}

// ServeJSonFile serves the content of the JSON schema along with the uiSchema if any is present
func ServeJSonFile(resourceName string) ([]byte, []byte, error) {
	schemaLocation := getSchemaMap()[resourceName]
	if schemaLocation == "" {
		return nil, nil, fmt.Errorf("requested resource's (%s) schema is not found", resourceName)
	}

	jsonContent, err := Schemas.ReadFile(schemaLocation)

	if err != nil {
		return nil, nil, fmt.Errorf("error reading json file: %s", err)
	}

	uiSchemaLocation := getUiSchemaMap()[resourceName]
	if uiSchemaLocation == "" {
		return jsonContent, nil, nil
	}

	uiSchemaJsonContent, err := Schemas.ReadFile(uiSchemaLocation)
	if err != nil {
		return jsonContent, nil, nil
	}

	return jsonContent, uiSchemaJsonContent, nil
}
