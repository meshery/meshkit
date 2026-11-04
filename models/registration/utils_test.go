package registration

import (
	"testing"

	"github.com/meshery/meshkit/models/meshmodel/entity"
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

func TestGetEntityAcceptsV1Beta3RelationshipSchemaVersion(t *testing.T) {
	t.Parallel()

	relationshipDocument := []byte(`{
		"schemaVersion": "relationships.meshery.io/v1beta3",
		"kind": "edge",
		"type": "non-binding",
		"subType": "reference",
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

func TestGetEntityRejectsUnknownRelationshipSchemaVersion(t *testing.T) {
	t.Parallel()

	relationshipDocument := []byte(`{
		"schemaVersion": "relationships.meshery.io/v1beta9",
		"kind": "edge",
		"type": "non-binding",
		"subType": "reference",
		"version": "v1.0.0"
	}`)

	_, err := getEntity(relationshipDocument)
	require.Error(t, err)
}
