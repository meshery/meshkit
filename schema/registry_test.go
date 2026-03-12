package schema

import (
	"testing"
	"testing/fstest"

	schemav1alpha3 "github.com/meshery/schemas/models/v1alpha3"
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

	require.Contains(t, byType, DocumentType("model"))
	require.Contains(t, byType, DocumentType("component"))
	require.Contains(t, byType, DocumentType("connection"))
	require.Contains(t, byType, DocumentType("design"))
	require.Contains(t, byType, DocumentType("relationship"))
	require.Contains(t, byType, DocumentType("environment"))

	assert.Equal(t, "schemas/constructs/v1beta1/model/model.yaml", byType[DocumentType("model")].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/component/component.yaml", byType[DocumentType("component")].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/connection/connection.yaml", byType[DocumentType("connection")].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/design/design.yaml", byType[DocumentType("design")].Location)
	assert.Equal(t, "schemas/constructs/v1alpha3/relationship/relationship.yaml#/components/schemas/RelationshipDefinition", byType[DocumentType("relationship")].Location)
	assert.Equal(t, "schemas/constructs/v1beta1/environment/environment.yaml", byType[DocumentType("environment")].Location)
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

	assert.Equal(t, schemav1beta1.ModelSchemaVersion, actual[DocumentType("model")])
	assert.Equal(t, schemav1beta1.ComponentSchemaVersion, actual[DocumentType("component")])
	assert.Equal(t, schemav1beta1.ConnectionSchemaVersion, actual[DocumentType("connection")])
	assert.Equal(t, schemav1alpha3.RelationshipSchemaVersion, actual[DocumentType("relationship")])
	assert.Empty(t, actual[DocumentType("design")])
	assert.Equal(t, "environments.meshery.io/v1beta1", actual[DocumentType("environment")])
}

func TestValidatorResolveUsesSchemaVersionConventionFallback(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	designRegistration, err := validator.resolve(Ref{SchemaVersion: schemav1beta1.DesignSchemaVersion})
	require.NoError(t, err)
	assert.Equal(t, DocumentType("design"), designRegistration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/design/design.yaml", designRegistration.Location)
	assert.Equal(t, "v1beta1", designRegistration.AssetVersion)

	detectedType := validator.documentTypeFromSchemaVersion(schemav1beta1.DesignSchemaVersion)
	assert.Equal(t, DocumentType("design"), detectedType)
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
			expectedType:  DocumentType("design"),
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
	assert.Equal(t, DocumentType("environment"), registration.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/environment/environment.yaml", registration.Location)
}

func TestValidatorDocumentTypesIncludesBuiltinTypes(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	types := validator.DocumentTypes()
	require.NotEmpty(t, types)

	typeSet := make(map[DocumentType]struct{}, len(types))
	for _, dt := range types {
		typeSet[dt] = struct{}{}
	}

	for _, expected := range []DocumentType{
		DocumentType("component"), DocumentType("connection"), DocumentType("design"),
		DocumentType("environment"), DocumentType("model"), DocumentType("relationship"),
	} {
		assert.Contains(t, typeSet, expected, "expected built-in type %q in DocumentTypes()", expected)
	}
}

func TestValidatorDocumentTypesIncludesDynamicTypes(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	types := validator.DocumentTypes()
	typeSet := make(map[DocumentType]struct{}, len(types))
	for _, dt := range types {
		typeSet[dt] = struct{}{}
	}

	assert.Contains(t, typeSet, DocumentType("selector"),
		"expected dynamically discovered type %q in DocumentTypes()", "selector")
	assert.Contains(t, typeSet, DocumentType("subcategory"),
		"expected dynamically discovered type %q in DocumentTypes()", "subcategory")
}

func TestValidatorDocumentTypesIsSorted(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	types := validator.DocumentTypes()
	require.NotEmpty(t, types)

	for i := 1; i < len(types); i++ {
		assert.LessOrEqual(t, string(types[i-1]), string(types[i]),
			"DocumentTypes() result is not sorted at index %d", i)
	}
}

func TestValidatorDocumentTypesAfterCustomRegistration(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	// Register a custom type with an asset version.
	customType := DocumentType("capability")
	err = validator.Register(Registration{
		Ref:          Ref{Type: customType},
		Location:     "schemas/constructs/v1beta1/environment/environment.yaml",
		AssetVersion: "v1beta1",
	})
	require.NoError(t, err)

	types := validator.DocumentTypes()
	typeSet := make(map[DocumentType]struct{}, len(types))
	for _, dt := range types {
		typeSet[dt] = struct{}{}
	}

	assert.Contains(t, typeSet, customType,
		"expected custom type %q to appear in DocumentTypes() after Register()", customType)
}

func TestValidatorDocumentTypesAfterCustomRegistrationWithoutAssetVersion(t *testing.T) {
	validator, err := New()
	require.NoError(t, err)

	// Register a custom type without an AssetVersion and with a non-versioned
	// location so Register cannot derive one either. This exercises the
	// registrations scan fallback in DocumentTypes().
	customType := DocumentType("policy")
	err = validator.Register(Registration{
		Ref:      Ref{Type: customType},
		Location: "custom/policy/policy.yaml",
		// AssetVersion is intentionally omitted.
	})
	require.NoError(t, err)

	types := validator.DocumentTypes()
	typeSet := make(map[DocumentType]struct{}, len(types))
	for _, dt := range types {
		typeSet[dt] = struct{}{}
	}

	assert.Contains(t, typeSet, customType,
		"expected custom type %q to appear in DocumentTypes() after Register() even without explicit AssetVersion", customType)
}

func TestPackageLevelDocumentTypes(t *testing.T) {
	types := DocumentTypes()
	require.NotEmpty(t, types)

	typeSet := make(map[DocumentType]struct{}, len(types))
	for _, dt := range types {
		typeSet[dt] = struct{}{}
	}

	assert.Contains(t, typeSet, DocumentType("relationship"))
	assert.Contains(t, typeSet, DocumentType("model"))
}
