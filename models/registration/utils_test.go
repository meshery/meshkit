package registration

import (
	"testing"

	"github.com/meshery/meshkit/models/meshmodel/entity"
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
