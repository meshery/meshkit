package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/stretchr/testify/assert"
)

func TestUpdateCompDefinitionWithDefaultCapabilities(t *testing.T) {
	tests := []struct {
		name                    string
		csvCapabilities         string
		expectedCapabilitiesLen int
		shouldHaveDefaultCaps   bool
	}{
		{
			name:                    "Empty capabilities should get defaults",
			csvCapabilities:         "",
			expectedCapabilitiesLen: 3,
			shouldHaveDefaultCaps:   true,
		},
		{
			name:                    "Null capabilities should get defaults",
			csvCapabilities:         "null",
			expectedCapabilitiesLen: 3,
			shouldHaveDefaultCaps:   true,
		},
		{
			name:                    "Existing capabilities should be preserved",
			csvCapabilities:         `[{"displayName":"Custom Cap","kind":"test"}]`,
			expectedCapabilitiesLen: 1,
			shouldHaveDefaultCaps:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := ComponentCSV{
				Component:    "TestComponent",
				Capabilities: tt.csvCapabilities,
				Registrant:   "meshery",
				Model:        "test-model",
			}

			compDef := &component.ComponentDefinition{}

			err := comp.UpdateCompDefinition(compDef)

			assert.NoError(t, err)
			assert.NotNil(t, compDef.Capabilities)
			assert.Len(t, *compDef.Capabilities, tt.expectedCapabilitiesLen)

			if tt.shouldHaveDefaultCaps {
				capabilities := *compDef.Capabilities

				expectedNames := []string{"Styling", "Change Shape", "Compound Drag And Drop"}
				actualNames := make([]string, len(capabilities))
				for i, cap := range capabilities {
					actualNames[i] = cap.DisplayName
				}

				assert.ElementsMatch(t, expectedNames, actualNames)

				assert.Equal(t, "Styling", capabilities[0].DisplayName)
				assert.Equal(t, "mutate", capabilities[0].Kind)
				assert.Equal(t, "style", capabilities[0].Type)
			}
		})
	}
}

func TestGetMinimalUICapabilitiesFromSchema(t *testing.T) {
	capabilities, err := getMinimalUICapabilitiesFromSchema()

	assert.NoError(t, err)
	assert.Len(t, capabilities, 3)

	expectedNames := []string{"Styling", "Change Shape", "Compound Drag And Drop"}
	actualNames := make([]string, len(capabilities))
	for i, cap := range capabilities {
		actualNames[i] = cap.DisplayName
	}

	assert.ElementsMatch(t, expectedNames, actualNames)
}

func TestGetSVGForRelationship(t *testing.T) {
	tests := []struct {
		name             string
		model            ModelCSV
		relationship     RelationshipCSV
		expectedColorSVG string
		expectedWhiteSVG string
	}{
		{
			name: "Relationship with its own SVGs",
			model: ModelCSV{
				SVGColor: "<svg>model-color</svg>",
				SVGWhite: "<svg>model-white</svg>",
			},
			relationship: RelationshipCSV{
				KIND:    "edge",
				SubType: "binding",
				Styles:  `{"svgColor": "<svg>rel-color</svg>", "svgWhite": "<svg>rel-white</svg>"}`,
			},
			expectedColorSVG: "<svg>rel-color</svg>",
			expectedWhiteSVG: "<svg>rel-white</svg>",
		},
		{
			name: "Relationship falls back to model SVGs",
			model: ModelCSV{
				SVGColor: "<svg>model-color</svg>",
				SVGWhite: "<svg>model-white</svg>",
			},
			relationship: RelationshipCSV{
				KIND:    "edge",
				SubType: "binding",
				Styles:  `{}`,
			},
			expectedColorSVG: "<svg>model-color</svg>",
			expectedWhiteSVG: "<svg>model-white</svg>",
		},
		{
			name: "Relationship with no styles uses model SVGs",
			model: ModelCSV{
				SVGColor: "<svg>model-color</svg>",
				SVGWhite: "<svg>model-white</svg>",
			},
			relationship: RelationshipCSV{
				KIND:    "edge",
				SubType: "binding",
				Styles:  "",
			},
			expectedColorSVG: "<svg>model-color</svg>",
			expectedWhiteSVG: "<svg>model-white</svg>",
		},
		{
			name: "Relationship with partial SVGs",
			model: ModelCSV{
				SVGColor: "<svg>model-color</svg>",
				SVGWhite: "<svg>model-white</svg>",
			},
			relationship: RelationshipCSV{
				KIND:    "edge",
				SubType: "binding",
				Styles:  `{"svgColor": "<svg>rel-color</svg>"}`,
			},
			expectedColorSVG: "<svg>rel-color</svg>",
			expectedWhiteSVG: "<svg>model-white</svg>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colorSVG, whiteSVG := getSVGForRelationship(tt.model, tt.relationship)
			assert.Equal(t, tt.expectedColorSVG, colorSVG)
			assert.Equal(t, tt.expectedWhiteSVG, whiteSVG)
		})
	}
}

func TestCreateRelationshipsMetadataAndCreateSVGsForMDXStyle(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "relationship-svg-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	model := ModelCSV{
		SVGColor: "<svg>model-color</svg>",
		SVGWhite: "<svg>model-white</svg>",
	}

	relationships := []RelationshipCSV{
		{
			KIND:        "edge",
			SubType:     "binding",
			Type:        "hierarchical",
			Description: "Test relationship",
			Styles:      `{"svgColor": "<svg>rel-color</svg>", "svgWhite": "<svg>rel-white</svg>"}`,
		},
	}

	svgDir := "icons"
	metadata, err := CreateRelationshipsMetadataAndCreateSVGsForMDXStyle(model, relationships, tmpDir, svgDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, metadata)

	// Verify metadata structure
	assert.Contains(t, metadata, "edge")
	assert.Contains(t, metadata, "hierarchical")
	assert.Contains(t, metadata, "Test relationship")

	// Verify SVG files were created - derive name the same way as implementation
	rel := relationships[0]
	relnshipName := utils.FormatName(manifests.FormatToReadableString(fmt.Sprintf("%s-%s", rel.KIND, rel.SubType)))
	colorSVGPath := filepath.Join(tmpDir, svgDir, relnshipName, "icons", "color", relnshipName+"-color.svg")
	whiteSVGPath := filepath.Join(tmpDir, svgDir, relnshipName, "icons", "white", relnshipName+"-white.svg")

	// Check color SVG exists and has correct content
	colorContent, err := os.ReadFile(colorSVGPath)
	assert.NoError(t, err)
	assert.Equal(t, "<svg>rel-color</svg>", string(colorContent))

	// Check white SVG exists and has correct content
	whiteContent, err := os.ReadFile(whiteSVGPath)
	assert.NoError(t, err)
	assert.Equal(t, "<svg>rel-white</svg>", string(whiteContent))
}

func TestCreateRelationshipsMetadataAndCreateSVGsForMDStyle(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "relationship-svg-test-md")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	model := ModelCSV{
		SVGColor: "<svg>model-color</svg>",
		SVGWhite: "<svg>model-white</svg>",
	}

	relationships := []RelationshipCSV{
		{
			KIND:        "edge",
			SubType:     "binding",
			Type:        "hierarchical",
			Description: "Test relationship",
			Styles:      `{"svgColor": "<svg>rel-color</svg>", "svgWhite": "<svg>rel-white</svg>"}`,
		},
	}

	svgDir := "icons"
	metadata, err := CreateRelationshipsMetadataAndCreateSVGsForMDStyle(model, relationships, tmpDir, svgDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, metadata)

	// Verify metadata structure
	assert.Contains(t, metadata, "edge")
	assert.Contains(t, metadata, "hierarchical")
	assert.Contains(t, metadata, "Test relationship")

	// Verify SVG files were created - derive name the same way as implementation
	rel := relationships[0]
	relnshipName := utils.FormatName(manifests.FormatToReadableString(fmt.Sprintf("%s-%s", rel.KIND, rel.SubType)))
	colorSVGPath := filepath.Join(tmpDir, relnshipName, "icons", "color", relnshipName+"-color.svg")
	whiteSVGPath := filepath.Join(tmpDir, relnshipName, "icons", "white", relnshipName+"-white.svg")

	// Check color SVG exists and has correct content
	colorContent, err := os.ReadFile(colorSVGPath)
	assert.NoError(t, err)
	assert.Equal(t, "<svg>rel-color</svg>", string(colorContent))

	// Check white SVG exists and has correct content
	whiteContent, err := os.ReadFile(whiteSVGPath)
	assert.NoError(t, err)
	assert.Equal(t, "<svg>rel-white</svg>", string(whiteContent))
}
