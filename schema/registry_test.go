package schema

import (
	"testing"
	"testing/fstest"

	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinRegistrationsDiscoverCoreSchemas(t *testing.T) {
	registrations, err := builtinRegistrations()
	require.NoError(t, err)
	require.NotEmpty(t, registrations)

	byType := map[DocumentType]Registration{}
	for _, registration := range registrations {
		byType[registration.Ref.Type] = registration
	}

	require.Contains(t, byType, TypeModel)
	require.Contains(t, byType, TypeComponent)
	require.Contains(t, byType, TypeConnection)
	require.Contains(t, byType, TypeDesign)
	require.Contains(t, byType, TypeRelationship)
	require.Contains(t, byType, TypeEnvironment)
	require.Contains(t, byType, TypeWorkspace)

	assert.Equal(t, "schemas/constructs/v1beta1/model/model.yaml", byType[TypeModel].Location)
	assert.Equal(t, "schemas/constructs/v1beta2/component/component.yaml", byType[TypeComponent].Location)
	assert.Equal(t, "schemas/constructs/v1beta2/connection/connection.yaml", byType[TypeConnection].Location)
	assert.Equal(t, "schemas/constructs/v1beta2/design/design.yaml", byType[TypeDesign].Location)
	assert.Equal(t, "schemas/constructs/v1beta2/relationship/relationship.yaml", byType[TypeRelationship].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/environment/environment.yaml", byType[TypeEnvironment].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/workspace/workspace.yaml", byType[TypeWorkspace].Location)
}

func TestBuiltinRegistrationsIncludeSelectorAndSubcategorySchemas(t *testing.T) {
	registrations, err := builtinRegistrations()
	require.NoError(t, err)

	var (
		foundSelector    bool
		foundSubcategory bool
	)

	for _, registration := range registrations {
		switch registration.Ref.Type {
		case DocumentType("selector"):
			foundSelector = true
		case DocumentType("subcategory"):
			foundSubcategory = true
		}
	}

	assert.True(t, foundSelector, "expected selector schemas to be discovered")
	assert.True(t, foundSubcategory, "expected subcategory schema to be discovered")
}

func TestBuiltinRegistrationsExtractSchemaVersionsWhenAvailable(t *testing.T) {
	registrations, err := builtinRegistrations()
	require.NoError(t, err)

	actual := map[DocumentType]string{}
	for _, registration := range registrations {
		actual[registration.Ref.Type] = registration.Ref.SchemaVersion
	}

	assert.Equal(t, "models.meshery.io/v1beta1", actual[TypeModel])
	assert.Equal(t, "components.meshery.io/v1beta2", actual[TypeComponent])
	assert.Equal(t, "connections.meshery.io/v1beta2", actual[TypeConnection])
	assert.Equal(t, "relationships.meshery.io/v1beta2", actual[TypeRelationship])
	assert.Equal(t, "designs.meshery.io/v1beta2", actual[TypeDesign])
	assert.Equal(t, "environments.meshery.io/v1beta1", actual[TypeEnvironment])
	assert.Empty(t, actual[TypeWorkspace])
}

func TestValidatorResolveUsesSchemaVersionConventionFallback(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	designRegistration, err := validator.resolve(Ref{SchemaVersion: schemav1beta1.DesignSchemaVersion})
	require.NoError(t, err)
	assert.Equal(t, TypeDesign, designRegistration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/design/design.yaml", designRegistration.Location)
	assert.Equal(t, "v1beta1", designRegistration.AssetVersion)

	detectedType := validator.documentTypeFromSchemaVersion(schemav1beta1.DesignSchemaVersion)
	assert.Equal(t, TypeDesign, detectedType)

	workspaceRegistration, err := validator.resolve(Ref{SchemaVersion: "workspaces.meshery.io/v1beta1"})
	require.NoError(t, err)
	assert.Equal(t, TypeWorkspace, workspaceRegistration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/workspace/workspace.yaml", workspaceRegistration.Location)
	assert.Equal(t, "v1beta1", workspaceRegistration.AssetVersion)

	detectedType = validator.documentTypeFromSchemaVersion("workspaces.meshery.io/v1beta1")
	assert.Equal(t, TypeWorkspace, detectedType)
}

func TestParseSchemaVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		schemaVersion string
		expectedType  DocumentType
		expectedAsset string
		expectedOK    bool
	}{
		{
			name:          "pluralized meshery schema version",
			schemaVersion: "designs.meshery.io/v1beta1",
			expectedType:  TypeDesign,
			expectedAsset: "v1beta1",
			expectedOK:    true,
		},
		{
			name:          "singular meshery schema version",
			schemaVersion: "capability.meshery.io/v1alpha1",
			expectedType:  "capability",
			expectedAsset: "v1alpha1",
			expectedOK:    true,
		},
		{
			name:          "pluralized workspace schema version",
			schemaVersion: "workspaces.meshery.io/v1beta1",
			expectedType:  TypeWorkspace,
			expectedAsset: "v1beta1",
			expectedOK:    true,
		},
		{
			name:          "unsupported bare version",
			schemaVersion: "v1alpha2",
			expectedOK:    false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			actualType, actualAsset, actualOK := parseSchemaVersion(testCase.schemaVersion)
			assert.Equal(t, testCase.expectedType, actualType)
			assert.Equal(t, testCase.expectedAsset, actualAsset)
			assert.Equal(t, testCase.expectedOK, actualOK)
		})
	}
}

func TestDiscoverRegistrationRecognizesNonObjectRootSchemas(t *testing.T) {
	t.Parallel()

	registration, ok, err := discoverRegistration(fstest.MapFS{
		"schemas/constructs/v1beta1/subcategory/subcategory.yaml": &fstest.MapFile{
			Data: []byte(`
$schema: http://json-schema.org/draft-07/schema#
type: string
enum:
  - Uncategorized
`),
		},
	}, "schemas/constructs/v1beta1/subcategory/subcategory.yaml")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, DocumentType("subcategory"), registration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/subcategory/subcategory.yaml", registration.Location)
}

func TestDiscoverRegistrationRecognizesSelectorStyleRootSchemas(t *testing.T) {
	t.Parallel()

	registration, ok, err := discoverRegistration(fstest.MapFS{
		"schemas/constructs/v1alpha3/selector/selector.yaml": &fstest.MapFile{
			Data: []byte(`
$schema: http://json-schema.org/draft-07/schema#
definitions:
  selector:
    type: array
`),
		},
	}, "schemas/constructs/v1alpha3/selector/selector.yaml")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, DocumentType("selector"), registration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1alpha3/selector/selector.yaml", registration.Location)
}

func TestValidatorResolvesEnvironmentBySchemaVersion(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	registration, err := validator.resolve(Ref{SchemaVersion: "environments.meshery.io/v1beta1"})
	require.NoError(t, err)
	assert.Equal(t, TypeEnvironment, registration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/environment/environment.yaml", registration.Location)
}
