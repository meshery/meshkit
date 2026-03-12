package schema

import (
	"encoding/json"
	"math"
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

const invalidMinimalModelDocument = `
schemaVersion: models.meshery.io/v1beta1
`

const validDesignDocument = `
id: 11111111-1111-1111-1111-111111111111
name: sample-design
schemaVersion: designs.meshery.io/v1beta1
version: v0.0.1
components: []
relationships: []
`

func TestDetectRef(t *testing.T) {
	ref, err := DetectRef([]byte("schemaVersion: relationships.meshery.io/v1alpha3"))
	require.NoError(t, err)

	assert.Equal(t, Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	}, ref)
}

func TestValidatorValidateRelationshipSuccess(t *testing.T) {
	err := Default().Validate([]byte(validRelationshipDocument))
	require.NoError(t, err)
}

func TestValidatorValidateDesignSuccess(t *testing.T) {
	err := Default().Validate([]byte(validDesignDocument))
	require.NoError(t, err)
}

func TestValidatorValidateRelationshipFailure(t *testing.T) {
	invalidRelationshipDocument := strings.Replace(validRelationshipDocument, "kind: edge", "kind: invalid", 1)
	expectedRegistration, err := Default().resolve(Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	})
	require.NoError(t, err)

	err = Default().Validate([]byte(invalidRelationshipDocument))
	require.Error(t, err)

	var meshkitErr *meshkiterrors.ErrorV2
	require.ErrorAs(t, err, &meshkitErr)
	assert.Equal(t, ErrValidateDocumentCode, meshkitErr.Code)

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	assert.Equal(t, Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	}, details.Ref)
	assert.Equal(t, expectedRegistration.Location, details.SchemaLocation)
	require.NotEmpty(t, details.Violations)

	instancePaths := make([]string, 0, len(details.Violations))
	for _, violation := range details.Violations {
		instancePaths = append(instancePaths, violation.InstancePath)
	}

	assert.Contains(t, instancePaths, "/kind")

	keywords := make([]string, 0, len(details.Violations))
	for _, violation := range details.Violations {
		keywords = append(keywords, violation.Keyword)
	}

	assert.Contains(t, keywords, "enum")
}

func TestValidatorCompileModelSchema(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	registration, err := validator.resolve(Ref{SchemaVersion: schemav1beta1.ModelSchemaVersion})
	require.NoError(t, err)

	_, err = validator.compile(registration.Location)
	require.NoError(t, err)
}

func TestValidatorValidateModelWithExplicitRefReportsViolations(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)
	expectedRegistration, err := validator.resolve(Ref{
		SchemaVersion: schemav1beta1.ModelSchemaVersion,
		Type:          DocumentType("model"),
	})
	require.NoError(t, err)

	err = validator.ValidateBytes(Ref{
		SchemaVersion: schemav1beta1.ModelSchemaVersion,
		Type:          DocumentType("model"),
	}, []byte(invalidMinimalModelDocument))
	require.Error(t, err)

	assert.Equal(t, ErrValidateDocumentCode, meshkiterrors.GetCode(err))

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	assert.Equal(t, expectedRegistration.Location, details.SchemaLocation)
	require.NotEmpty(t, details.Violations)
}

func TestValidatorValidateUnsupportedSchemaVersion(t *testing.T) {
	err := Default().Validate([]byte("schemaVersion: unsupported.meshery.io/v1alpha1"))
	require.Error(t, err)
	assert.Equal(t, ErrResolveSchemaCode, meshkiterrors.GetCode(err))
}

func TestValidatorResolveDoesNotFallbackFromUnknownSchemaVersionToType(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	_, err = validator.resolve(Ref{
		SchemaVersion: "unsupported.meshery.io/v1alpha1",
		Type:          DocumentType("model"),
	})
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
			Type:          DocumentType("relationship"),
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
	validator, err := New()
	require.NoError(t, err)

	document, err := DecodeAndValidateWithValidator[map[string]any](validator, Ref{}, []byte(validRelationshipDocument))
	require.NoError(t, err)
	assert.Equal(t, schemav1alpha3.RelationshipSchemaVersion, document["schemaVersion"])
}

func TestValidatorValidateAnyWithZeroRef(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	var document map[string]any
	err = meshkitencoding.Unmarshal([]byte(validRelationshipDocument), &document)
	require.NoError(t, err)

	err = validator.ValidateAny(Ref{}, document)
	require.NoError(t, err)
}

func TestValidatorValidateAnyWithZeroRefAndNonStringSchemaVersion(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	err = validator.ValidateAny(Ref{}, map[string]any{
		"schemaVersion": 1,
	})
	require.Error(t, err)
	assert.Equal(t, ErrDecodeDocumentCode, meshkiterrors.GetCode(err))
}

func TestValidatorDocumentTypeFromSchemaVersionUsesRegistrations(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)
	environmentRegistration, err := validator.resolve(Ref{Type: DocumentType("environment")})
	require.NoError(t, err)

	require.NoError(t, validator.Register(Registration{
		Ref: Ref{
			SchemaVersion: "example.meshery.io/v1alpha1",
			Type:          DocumentType("environment"),
		},
		Location:     environmentRegistration.Location,
		AssetVersion: environmentRegistration.AssetVersion,
	}))

	assert.Equal(t, DocumentType("environment"), validator.documentTypeFromSchemaVersion("example.meshery.io/v1alpha1"))
}

func TestValidateAnyWithExplicitRef(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	var document map[string]any
	err = meshkitencoding.Unmarshal([]byte(validRelationshipDocument), &document)
	require.NoError(t, err)

	err = validator.ValidateAny(Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	}, document)
	require.NoError(t, err)
}

func TestDecodeAndValidateWithRefSuccess(t *testing.T) {
	ref := Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	}
	document, err := DecodeAndValidateWithRef[map[string]any](
		ref,
		[]byte(validRelationshipDocument),
	)
	require.NoError(t, err)
	assert.Equal(t, schemav1alpha3.RelationshipSchemaVersion, document["schemaVersion"])
}

func TestDecodeAndValidateWithValidatorDecodeFailure(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	type BadTarget struct {
		ID int `json:"id"`
	}

	_, err = DecodeAndValidateWithValidator[BadTarget](validator, Ref{
		SchemaVersion: schemav1alpha3.RelationshipSchemaVersion,
		Type:          DocumentType("relationship"),
	}, []byte(validRelationshipDocument))
	require.Error(t, err)
	assert.Equal(t, ErrDecodeTypedDocumentCode, meshkiterrors.GetCode(err))
}

func TestNormalizeDocumentPreservesScalarTypes(t *testing.T) {
	document := map[string]any{
		"int":    int64(42),
		"uint":   uint64(math.MaxUint64),
		"number": json.Number("18446744073709551615"),
		"nested": []any{int32(7), map[string]any{"float": 3.14}},
	}

	normalized, err := normalizeDocument(document)
	require.NoError(t, err)

	normalizedMap, ok := normalized.(map[string]any)
	require.True(t, ok)

	assert.IsType(t, int64(0), normalizedMap["int"])
	assert.EqualValues(t, uint64(math.MaxUint64), normalizedMap["uint"])
	assert.IsType(t, json.Number(""), normalizedMap["number"])

	nested, ok := normalizedMap["nested"].([]any)
	require.True(t, ok)
	assert.IsType(t, int32(0), nested[0])
}
