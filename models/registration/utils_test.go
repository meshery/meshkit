package registration

import (
	"testing"

	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/schema"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta3/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEntityAcceptsV1Beta2RelationshipSchemaVersion(t *testing.T) {
	t.Parallel()

	relationshipDocument := []byte(`{
		"schemaVersion": "relationships.meshery.io/v1beta2",
		"kind": "edge",
		"type": "binding",
		"subType": "firewall",
		"model": {
			"name": "kubernetes",
			"model": {
				"version": "v1.0.0"
			}
		},
		"version": "v1.0.0"
	}`)

	actual, err := getEntity(relationshipDocument)
	require.NoError(t, err)
	assert.Equal(t, entity.RelationshipDefinition, actual.Type())
}

// TestGetEntityAcceptsEveryRelationshipSchemaVersion covers v1alpha3, v1beta2
// and v1beta3 relationship documents. relationships.meshery.io/v1beta3 is what
// the `mesheryctl model init` scaffold carries; it was rejected at
// registration.
func TestGetEntityAcceptsEveryRelationshipSchemaVersion(t *testing.T) {
	t.Parallel()

	for _, schemaVersion := range []string{
		v1alpha3.RelationshipSchemaVersion,
		schema.RelationshipSchemaVersionV1Beta2,
		schema.RelationshipSchemaVersionV1Beta3,
	} {
		t.Run(schemaVersion, func(t *testing.T) {
			t.Parallel()

			relationshipDocument := []byte(`{
				"schemaVersion": "` + schemaVersion + `",
				"version": "v1.0.0",
				"kind": "edge",
				"type": "binding",
				"subType": "firewall",
				"status": "enabled",
				"model": {
					"name": "kubernetes",
					"model": {"version": "v1.0.0"}
				}
			}`)

			actual, err := getEntity(relationshipDocument)
			require.NoError(t, err)
			require.Equal(t, entity.RelationshipDefinition, actual.Type())
		})
	}
}

// TestGetEntityV1Beta3RelationshipKeepsFields checks the converted v1beta3
// relationship keeps its identifying fields.
func TestGetEntityV1Beta3RelationshipKeepsFields(t *testing.T) {
	t.Parallel()

	doc := []byte(`{
		"schemaVersion": "relationships.meshery.io/v1beta3",
		"version": "v1.0.0",
		"kind": "edge",
		"type": "non-binding",
		"subType": "network",
		"status": "enabled",
		"metadata": {"description": "sample relationship"},
		"model": {"name": "kubernetes", "model": {"version": "v1.0.0"}}
	}`)

	actual, err := getEntity(doc)
	require.NoError(t, err)

	converted, ok := actual.(*relationship.RelationshipDefinition)
	require.True(t, ok, "expected *relationship.RelationshipDefinition, got %T", actual)
	assert.EqualValues(t, "edge", converted.Kind)
	assert.EqualValues(t, "v1.0.0", converted.Version)
	assert.EqualValues(t, "network", converted.SubType)
	assert.Equal(t, "kubernetes", converted.Model.Name)
}

func TestFindLostFieldsReportsDroppedFields(t *testing.T) {
	t.Parallel()

	src := map[string]any{
		"kind":     "edge",
		"extra":    "value",
		"metadata": map[string]any{"description": "d", "unknown": "x"},
		"model":    map[string]any{"name": "kubernetes"},
	}
	dst := map[string]any{
		"kind":     "edge",
		"metadata": map[string]any{"description": "d"},
		"model":    map[string]any{"name": "kubernetes"},
	}

	assert.Equal(t, []string{"extra", "metadata.unknown"}, findLostFields(src, dst))
	assert.Empty(t, findLostFields(dst, dst))
}

func TestFindLostFieldsDetectsChangedExplicitFalse(t *testing.T) {
	t.Parallel()

	src := map[string]any{"enabled": false, "count": float64(0), "flag": false}

	changed := map[string]any{"enabled": true, "count": float64(2)}
	assert.Equal(t, []string{"count", "enabled"}, findLostFields(src, changed))

	unchanged := map[string]any{"enabled": false, "count": float64(0)}
	assert.Empty(t, findLostFields(src, unchanged))
}

func TestRenameKeysMapsV1Beta3Names(t *testing.T) {
	t.Parallel()

	src := map[string]any{
		"metadata": map[string]any{"styles": map[string]any{
			"curveStyle": "bezier",
			"lineColor":  "#fff",
			"zIndex":     float64(2),
		}},
		"selectors": []any{map[string]any{"matchStrategyMatrix": []any{"a"}}},
	}

	assert.Equal(t, map[string]any{
		"metadata": map[string]any{"styles": map[string]any{
			"curve-style": "bezier",
			"line-color":  "#fff",
			"z-index":     float64(2),
		}},
		"selectors": []any{map[string]any{"match_strategy_matrix": []any{"a"}}},
	}, renameKeys(src))
}

func TestV1Beta3KeyRenamesCoverEveryDifferingKey(t *testing.T) {
	t.Parallel()

	assert.Len(t, v1beta3KeyRenames, 23)
	for from, to := range v1beta3KeyRenames {
		assert.NotEqual(t, from, to)
	}
}

// TestGetEntityAcceptsEveryComponentSchemaVersion pins the component schema
// versions registration accepts. components.meshery.io/v1beta3 is the string
// the v1beta3 component construct and its templates (the `mesheryctl model
// init` scaffold) carry; it was rejected with "Not a valid component
// definition", so no scaffolded model could be imported.
func TestGetEntityAcceptsEveryComponentSchemaVersion(t *testing.T) {
	t.Parallel()

	for _, schemaVersion := range []string{
		"components.meshery.io/v1beta1",
		"components.meshery.io/v1beta2",
		"components.meshery.io/v1beta3",
	} {
		t.Run(schemaVersion, func(t *testing.T) {
			t.Parallel()

			componentDocument := []byte(`{
				"schemaVersion": "` + schemaVersion + `",
				"version": "v0.1.0",
				"displayName": "KubeCon Logo",
				"format": "JSON",
				"model": {
					"name": "kubecon-logo",
					"version": "v0.1.0",
					"model": {"version": "v0.1.0"}
				},
				"metadata": {"isAnnotation": true},
				"component": {"kind": "kubecon-logo", "version": "v0.1.0", "schema": ""}
			}`)

			actual, err := getEntity(componentDocument)
			require.NoError(t, err)
			require.Equal(t, entity.ComponentDefinition, actual.Type())

			comp, ok := actual.(*component.ComponentDefinition)
			require.True(t, ok, "expected *component.ComponentDefinition, got %T", actual)
			assert.Equal(t, "kubecon-logo", comp.Component.Kind)
			assert.Equal(t, "kubecon-logo", comp.Model.Name)
		})
	}
}
