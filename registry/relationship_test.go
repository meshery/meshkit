package registry

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessRelationshipsWritesV1Beta2SchemaVersion(t *testing.T) {
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
	assert.Equal(t, schema.RelationshipSchemaVersionV1Beta2, document["schemaVersion"])
}

func TestProcessRelationshipsSanitizesFilenameFromSpreadsheet(t *testing.T) {
	Log = SetupLogger("relationship-test", false, io.Discard)

	tests := []struct {
		name     string
		filename string
		expected []string
	}{
		{
			name:     "invisible left-to-right mark after the extension is removed",
			filename: "edge-binding-firewall.json‎",
			expected: []string{"edge-binding-firewall.json"},
		},
		{
			name:     "surrounding spaces are removed",
			filename: "  edge-binding-firewall.json ",
			expected: []string{"edge-binding-firewall.json"},
		},
		{
			name:     "parent directory segments are dropped",
			filename: "../../edge-binding-firewall.json",
			expected: []string{"edge-binding-firewall.json"},
		},
		{
			name:     "windows separators are dropped",
			filename: `..\..\edge-binding-firewall.json`,
			expected: []string{"edge-binding-firewall.json"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outputDir := t.TempDir()
			definitionDir := filepath.Join(outputDir, "kubernetes", "v1.0.0", "v1.0.0")
			require.NoError(t, os.MkdirAll(definitionDir, 0o755))

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
						Filename:          tc.filename,
					},
				},
			}

			ProcessRelationships(helper, make(chan RelationshipCSV, 1), outputDir)

			entries, err := os.ReadDir(filepath.Join(definitionDir, "relationships"))
			require.NoError(t, err)
			var names []string
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			assert.Equal(t, tc.expected, names)

			// nothing may be written outside the relationships folder
			outside, err := os.ReadDir(definitionDir)
			require.NoError(t, err)
			require.Len(t, outside, 1)
			assert.Equal(t, "relationships", outside[0].Name())
		})
	}
}

func TestSanitizeRelationshipFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{name: "plain name is unchanged", filename: "edge-binding-firewall.json", expected: "edge-binding-firewall.json"},
		{name: "empty stays empty", filename: "", expected: ""},
		{name: "only an invisible character", filename: "‎", expected: ""},
		{name: "only spaces", filename: "   ", expected: ""},
		{name: "current directory", filename: ".", expected: ""},
		{name: "parent directory", filename: "..", expected: ""},
		{name: "trailing separator keeps the last segment", filename: "relationships/edge.json/", expected: "edge.json"},
		{name: "zero width space inside the name", filename: "edge-​binding.json", expected: "edge-binding.json"},
		{name: "non-ASCII letters are kept", filename: "édge-binding.json", expected: "édge-binding.json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeRelationshipFilename(tc.filename))
		})
	}
}
