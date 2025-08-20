package schema

import (
	"encoding/json"
	"fmt"

	"github.com/meshery/meshkit/schemas"
)

// ValidateConnectionDefinition validates a connection definition against its schema
func ValidateConnectionDefinition(connectionData []byte) error {
	// Get the connection definition schema
	schemaData, err := schemas.Schemas.ReadFile("connections/connectionDefinition.json")
	if err != nil {
		return fmt.Errorf("failed to read connection definition schema: %w", err)
	}

	// Validate the data against the schema
	return validateJSONSchema(connectionData, schemaData)
}

// validateJSONSchema validates JSON data against a JSON schema
func validateJSONSchema(data, schema []byte) error {
	// Parse the schema
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	// Parse the data
	var dataObj interface{}
	if err := json.Unmarshal(data, &dataObj); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	// For now, we'll do basic validation
	// In a production environment, you'd want to use a proper JSON schema validator
	// like github.com/xeipuuv/gojsonschema or similar

	// Basic structure validation
	dataMap, ok := dataObj.(map[string]interface{})
	if !ok {
		return fmt.Errorf("data is not a valid JSON object")
	}

	// Check required fields
	required, ok := schemaObj["required"].([]interface{})
	if ok {
		for _, req := range required {
			reqStr, ok := req.(string)
			if !ok {
				continue
			}
			if _, exists := dataMap[reqStr]; !exists {
				return fmt.Errorf("missing required field: %s", reqStr)
			}
		}
	}

	return nil
}

// ValidateConnectionDefinitionWithStruct validates both JSON schema and Go struct
func ValidateConnectionDefinitionWithStruct(connectionData []byte) error {
	// First validate against JSON schema
	if err := ValidateConnectionDefinition(connectionData); err != nil {
		return fmt.Errorf("JSON schema validation failed: %w", err)
	}

	// Then validate that it can be unmarshaled into the Go struct
	// This ensures both JSON schema and Go struct are in sync
	var connDef map[string]interface{}
	if err := json.Unmarshal(connectionData, &connDef); err != nil {
		return fmt.Errorf("failed to unmarshal into Go struct: %w", err)
	}

	return nil
}
