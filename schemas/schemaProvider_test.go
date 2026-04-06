package schemas

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeJSonFileWorkspace(t *testing.T) {
	schema, uiSchema, err := ServeJSonFile("workspace")
	require.NoError(t, err)
	require.NotEmpty(t, schema)
	require.NotEmpty(t, uiSchema)

	assert.True(t, bytes.Contains(schema, []byte(`"title": "Workspace"`)))
	assert.True(t, bytes.Contains(schema, []byte(`"organization"`)))
	assert.True(t, bytes.Contains(uiSchema, []byte(`"organization"`)))
}
