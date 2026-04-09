package schemas

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeJSONFileWorkspace(t *testing.T) {
	schema, uiSchema, err := ServeJSonFile("workspace")
	require.NoError(t, err)
	require.NotEmpty(t, schema)
	require.NotEmpty(t, uiSchema)

	var schemaDocument map[string]any
	require.NoError(t, json.Unmarshal(schema, &schemaDocument))
	assert.Equal(t, "Workspace", schemaDocument["title"])

	properties, ok := schemaDocument["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "organization")

	var uiSchemaDocument map[string]any
	require.NoError(t, json.Unmarshal(uiSchema, &uiSchemaDocument))
	assert.Contains(t, uiSchemaDocument, "organization")
}
