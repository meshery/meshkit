package registry

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/meshery/meshkit/encoding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessRelationshipsWritesV1Beta2SchemaVersion(t *testing.T) {
	t.Parallel()

	Log = SetupLogger("relationship-test", false, io.Discard)

	outputDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outputDir, "kubernetes", "v1.0.0", "v1.0.0"), 0o755))

	helper := &RelationshipCSVHelper{
		Relationships: []RelationshipCSV{
			{
				Model:             "kubernetes",
				Version:           "v1.0.0",
				KIND:              "edge",
				Type:              "binding",
				Status:            "approved",
				SubType:           "firewall",
				PublishToRegistry: "true",
				Filename:          "edge-binding-firewall.json",
			},
		},
	}

	ProcessRelationships(helper, make(chan RelationshipCSV, 1), outputDir)

	relationshipPath := filepath.Join(outputDir, "kubernetes", "v1.0.0", "v1.0.0", "relationships", "edge-binding-firewall.json")
	relationshipBytes, err := os.ReadFile(relationshipPath)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, encoding.Unmarshal(relationshipBytes, &document))
	assert.Equal(t, relationshipSchemaVersionV1Beta2, document["schemaVersion"])
}
