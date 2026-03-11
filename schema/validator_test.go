package schema

import (
	"strings"
	"testing"

	meshkitencoding "github.com/meshery/meshkit/encoding"
	meshkiterrors "github.com/meshery/meshkit/errors"
	schemav1alpha3 "github.com/meshery/schemas/models/v1alpha3"
	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validRelationshipDocument = `
id: 11111111-1111-1111-1111-111111111111
schemaVersion: relationships.meshery.io/v1alpha3
version: v1.0.0
kind: edge
type: binding
subType: firewall
model:
  id: 22222222-2222-2222-2222-222222222222
  name: kubernetes
  version: v1.0.0
  displayName: Kubernetes
  model:
    version: v1.0.0
  registrant:
    kind: github
`

func TestDetectRef(t *testing.T) {
	ref, err := DetectRef([]byte("schemaVersion: relationships.meshery.io/v1alpha3"))
	require.NoError(t, err)

	assert.Equal(t, Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          TypeRelationship,
	}, ref)
}

func TestValidatorValidateRelationshipSuccess(t *testing.T) {
	err := Default().Validate([]byte(validRelationshipDocument))
	require.NoError(t, err)
}

func TestValidatorValidateRelationshipFailure(t *testing.T) {
	invalidRelationshipDocument := strings.Replace(validRelationshipDocument, "kind: edge", "kind: invalid", 1)

	err := Default().Validate([]byte(invalidRelationshipDocument))
	require.Error(t, err)

	var meshkitErr *meshkiterrors.ErrorV2
	require.ErrorAs(t, err, &meshkitErr)
	assert.Equal(t, ErrValidateDocumentCode, meshkitErr.Code)

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	assert.Equal(t, Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          TypeRelationship,
	}, details.Ref)
	assert.Equal(t, relationshipSchemaLocation, details.SchemaLocation)
	require.NotEmpty(t, details.Violations)

	instancePaths := make([]string, 0, len(details.Violations))
	for _, violation := range details.Violations {
		instancePaths = append(instancePaths, violation.InstancePath)
	}

	assert.Contains(t, instancePaths, "/kind")
}

func TestValidatorCompileModelSchema(t *testing.T) {
	validator := New()

	registration, err := validator.resolve(Ref{SchemaVersion: schemav1beta1.ModelSchemaVersion})
	require.NoError(t, err)

	_, err = validator.compile(registration.Location)
	require.NoError(t, err)
}

func TestValidatorValidateUnsupportedSchemaVersion(t *testing.T) {
	err := Default().Validate([]byte("schemaVersion: unsupported.meshery.io/v1alpha1"))
	require.Error(t, err)
	assert.Equal(t, ErrResolveSchemaCode, meshkiterrors.GetCode(err))
}

func TestValidatorValidateRelationshipWithMismatchedSchemaVersion(t *testing.T) {
	mismatchedSchemaVersionDocument := strings.Replace(
		validRelationshipDocument,
		"schemaVersion: relationships.meshery.io/v1alpha3",
		"schemaVersion: models.meshery.io/v1beta1",
		1,
	)

	err := Default().ValidateBytes(
		Ref{
			SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
			Type:          TypeRelationship,
		},
		[]byte(mismatchedSchemaVersionDocument),
	)
	require.Error(t, err)

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	require.Len(t, details.Violations, 1)
	assert.Equal(t, "/schemaVersion", details.Violations[0].InstancePath)
	assert.Equal(t, "const", details.Violations[0].Keyword)
}

func TestDecodeAndValidateWithValidatorZeroRef(t *testing.T) {
	validator := New()

	document, err := DecodeAndValidateWithValidator[map[string]any](validator, Ref{}, []byte(validRelationshipDocument))
	require.NoError(t, err)
	assert.Equal(t, schemav1alpha3.RelationshipSchemaVersion, document["schemaVersion"])
}

func TestValidatorValidateAnyWithZeroRef(t *testing.T) {
	validator := New()

	var document map[string]any
	err := meshkitencoding.Unmarshal([]byte(validRelationshipDocument), &document)
	require.NoError(t, err)

	err = validator.ValidateAny(Ref{}, document)
	require.NoError(t, err)
}

func TestValidatorValidateAnyWithZeroRefAndNonStringSchemaVersion(t *testing.T) {
	validator := New()

	err := validator.ValidateAny(Ref{}, map[string]any{
		"schemaVersion": 1,
	})
	require.Error(t, err)
	assert.Equal(t, ErrDecodeDocumentCode, meshkiterrors.GetCode(err))
}

func TestValidatorDocumentTypeFromSchemaVersionUsesRegistrations(t *testing.T) {
	validator := New()

	require.NoError(t, validator.Register(Registration{
		Ref: Ref{
			SchemaVersion: "example.meshery.io/v1alpha1",
			Type:          TypeEnvironment,
		},
		Location: environmentSchemaLocation,
	}))

	assert.Equal(t, TypeEnvironment, validator.documentTypeFromSchemaVersion("example.meshery.io/v1alpha1"))
}
