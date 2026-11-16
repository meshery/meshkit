package registration

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/schema"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta3/component"
	relationshipv1beta3 "github.com/meshery/schemas/models/v1beta3/relationship"
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
		"metadata": {
			"description": "sample relationship",
			"lineColor": "#custom",
			"styles": {"curveStyle": "bezier", "lineColor": "#fff"}
		},
		"selectors": [{
			"allow": {
				"from": [{"kind": "Deployment", "matchStrategyMatrix": [["kind"]]}],
				"to": [{"kind": "Service"}]
			}
		}],
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

	raw, err := json.Marshal(converted)
	require.NoError(t, err)
	var roundTripped map[string]any
	require.NoError(t, json.Unmarshal(raw, &roundTripped))

	meta, _ := roundTripped["metadata"].(map[string]any)
	assert.Equal(t, "#custom", meta["lineColor"])
	styles, _ := meta["styles"].(map[string]any)
	assert.Equal(t, "bezier", styles["curve-style"])
	assert.Equal(t, "#fff", styles["line-color"])

	selectors, _ := roundTripped["selectors"].([]any)
	require.Len(t, selectors, 1)
	allow, _ := selectors[0].(map[string]any)["allow"].(map[string]any)
	from, _ := allow["from"].([]any)
	require.Len(t, from, 1)
	assert.Equal(t, []any{[]any{"kind"}}, from[0].(map[string]any)["match_strategy_matrix"])
}

// TestGetEntityV1Beta3RelationshipSurvivesExportReimport mirrors Meshery's
// ExportModel: the stored entity is written out and read back. The converted
// entity carries the v1alpha3 schemaVersion, so the second import takes the
// v1alpha3 path and nothing is dropped.
func TestGetEntityV1Beta3RelationshipSurvivesExportReimport(t *testing.T) {
	t.Parallel()

	doc := []byte(`{
		"schemaVersion": "relationships.meshery.io/v1beta3",
		"version": "v1.0.0",
		"kind": "edge",
		"type": "non-binding",
		"subType": "network",
		"status": "enabled",
		"metadata": {
			"styles": {"curveStyle": "bezier", "lineColor": "#fff"}
		},
		"selectors": [{
			"allow": {
				"from": [{"kind": "Deployment", "matchStrategyMatrix": [["kind"]]}],
				"to": [{"kind": "Service"}]
			}
		}],
		"model": {"name": "kubernetes", "model": {"version": "v1.0.0"}}
	}`)

	first, err := getEntity(doc)
	require.NoError(t, err)
	converted, ok := first.(*relationship.RelationshipDefinition)
	require.True(t, ok, "expected *relationship.RelationshipDefinition, got %T", first)
	assert.EqualValues(t, v1alpha3.RelationshipSchemaVersion, converted.SchemaVersion)

	exported, err := json.Marshal(converted)
	require.NoError(t, err)

	second, err := getEntity(exported)
	require.NoError(t, err)
	reimported, ok := second.(*relationship.RelationshipDefinition)
	require.True(t, ok, "expected *relationship.RelationshipDefinition, got %T", second)

	raw, err := json.Marshal(reimported)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))

	meta, _ := got["metadata"].(map[string]any)
	styles, _ := meta["styles"].(map[string]any)
	assert.Equal(t, "bezier", styles["curve-style"])
	assert.Equal(t, "#fff", styles["line-color"])

	selectors, _ := got["selectors"].([]any)
	require.Len(t, selectors, 1)
	allow, _ := selectors[0].(map[string]any)["allow"].(map[string]any)
	from, _ := allow["from"].([]any)
	require.Len(t, from, 1)
	assert.Equal(t, []any{[]any{"kind"}}, from[0].(map[string]any)["match_strategy_matrix"])
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
		"selectors": []any{map[string]any{
			"allow": map[string]any{
				"from": []any{map[string]any{"matchStrategyMatrix": []any{"a"}}},
			},
		}},
	}

	assert.Equal(t, map[string]any{
		"metadata": map[string]any{"styles": map[string]any{
			"curve-style": "bezier",
			"line-color":  "#fff",
			"z-index":     float64(2),
		}},
		"selectors": []any{map[string]any{
			"allow": map[string]any{
				"from": []any{map[string]any{"match_strategy_matrix": []any{"a"}}},
			},
		}},
	}, renameKeys(src))
}

func TestRenameKeysLeavesFreeFormMapsUntouched(t *testing.T) {
	t.Parallel()

	src := map[string]any{
		"capabilities": []any{map[string]any{
			"metadata": map[string]any{"lineColor": "app-data", "zIndex": float64(1)},
		}},
		"metadata": map[string]any{
			"styles": map[string]any{"lineColor": "#fff", "line-color": "#000"},
		},
	}

	assert.Equal(t, map[string]any{
		"capabilities": []any{map[string]any{
			"metadata": map[string]any{"lineColor": "app-data", "zIndex": float64(1)},
		}},
		"metadata": map[string]any{
			"styles": map[string]any{"lineColor": "#fff", "line-color": "#000"},
		},
	}, renameKeys(src))
}

// jsonTagNames collects every json tag name reachable from t, walking nested
// structs, slices, arrays, maps and pointers.
func jsonTagNames(t reflect.Type, seen map[reflect.Type]bool, out map[string]bool) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			out[name] = true
		}
		jsonTagNames(f.Type, seen, out)
	}
}

// TestV1Beta3KeyRenamesMatchSchemaTagDifferences walks the json tags of the
// v1alpha3 and v1beta3 relationship structs and asserts that the tags present
// on one side only are exactly the keys in the rename map. A schemas bump that
// adds, removes or renames a key on one side fails here in CI instead of in a
// user's import.
func TestV1Beta3KeyRenamesMatchSchemaTagDifferences(t *testing.T) {
	t.Parallel()

	alpha := map[string]bool{}
	beta := map[string]bool{}
	jsonTagNames(reflect.TypeOf(relationship.RelationshipDefinition{}), map[reflect.Type]bool{}, alpha)
	jsonTagNames(reflect.TypeOf(relationshipv1beta3.RelationshipDefinition{}), map[reflect.Type]bool{}, beta)

	wantBetaOnly := map[string]bool{v1beta3MatrixKey: true}
	wantAlphaOnly := map[string]bool{v1alpha3MatrixKey: true}
	for from, to := range v1beta3StyleKeyRenames {
		assert.NotEqual(t, from, to)
		wantBetaOnly[from] = true
		wantAlphaOnly[to] = true
	}

	gotBetaOnly := map[string]bool{}
	for k := range beta {
		if !alpha[k] {
			gotBetaOnly[k] = true
		}
	}
	gotAlphaOnly := map[string]bool{}
	for k := range alpha {
		if !beta[k] {
			gotAlphaOnly[k] = true
		}
	}

	assert.Equal(t, wantBetaOnly, gotBetaOnly)
	assert.Equal(t, wantAlphaOnly, gotAlphaOnly)
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
