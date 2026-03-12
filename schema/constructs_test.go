package schema

// constructs_test.go strengthens coverage for the two document families that
// illustrate the full spectrum of schema-discovery/validation behaviour:
//
//  1. Connections – a schema present in the released github.com/meshery/schemas
//     module, complete with a schemaVersion default, so that both auto-detection
//     and explicit-type validation work end-to-end today.
//
//  2. Credentials – no schema exists yet in the released module; the design is
//     based on the credential.yaml found on the meshery/schemas "credentials"
//     branch, which lacks a schemaVersion property.  These tests prove that:
//     • a credential-like schema asset is still discoverable and registers a
//       document type even when the schema has no schemaVersion property.
//     • once registered, an explicit Ref{Type: "credential"} can validate a
//       document that itself carries no schemaVersion field.
//     • without a schemaVersion field the document cannot be auto-detected, and
//       ErrDetectSchemaVersion is returned – documenting a known limitation.

import (
	"testing"
	"testing/fstest"

	meshkiterrors "github.com/meshery/meshkit/errors"
	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Connection documents
// ---------------------------------------------------------------------------

// validConnectionDocument is the minimal set of required fields for a
// connection document.  Keeping it lean avoids coupling the test to optional
// fields that may change between schema releases.
const validConnectionDocument = `
id: 11111111-1111-1111-1111-111111111111
name: test-connection
type: platform
sub_type: orchestration
kind: kubernetes
status: discovered
schemaVersion: connections.meshery.io/v1beta1
`

// TestValidatorValidateConnectionBySchemaVersionSuccess validates a well-formed
// connection document using auto-detection (schemaVersion field in the
// document drives schema selection).
func TestValidatorValidateConnectionBySchemaVersionSuccess(t *testing.T) {
	err := Default().Validate([]byte(validConnectionDocument))
	require.NoError(t, err,
		"valid connection document must pass schemaVersion-based auto-detection")
}

// TestValidatorValidateConnectionByExplicitTypeSuccess validates the same
// document using an explicit DocumentType ref, showing that connection
// validation works even when the caller bypasses auto-detection.
func TestValidatorValidateConnectionByExplicitTypeSuccess(t *testing.T) {
	err := ValidateAs(DocumentType("connection"), []byte(validConnectionDocument))
	require.NoError(t, err,
		"valid connection document must pass explicit-type validation")
}

// TestValidatorValidateConnectionSchemaVersionRegistered verifies that the
// connection schema is registered under the canonical schema version constant
// and under the "connection" document type.
func TestValidatorValidateConnectionSchemaVersionRegistered(t *testing.T) {
	v, err := New()
	require.NoError(t, err)

	byVersion, err := v.resolve(Ref{SchemaVersion: schemav1beta1.ConnectionSchemaVersion})
	require.NoError(t, err, "connection schema must be resolvable by schemaVersion constant")
	assert.Equal(t, DocumentType("connection"), byVersion.Ref.Type)
	assert.Equal(t, "schemas/constructs/v1beta1/connection/connection.yaml", byVersion.Location)

	byType, err := v.resolve(Ref{Type: DocumentType("connection")})
	require.NoError(t, err, "connection schema must be resolvable by document type")
	assert.Equal(t, byVersion.Location, byType.Location,
		"schemaVersion-based and type-based resolution must point to the same schema asset")
}

// TestValidatorValidateConnectionMissingRequiredFieldReportsViolations confirms
// that a connection document missing required fields fails validation and
// returns structured violation details.
func TestValidatorValidateConnectionMissingRequiredFieldReportsViolations(t *testing.T) {
	// Omit sub_type, kind, and status – all required by the connection schema.
	incompleteDoc := `
id: 11111111-1111-1111-1111-111111111111
name: test-connection
type: platform
schemaVersion: connections.meshery.io/v1beta1
`
	err := Default().Validate([]byte(incompleteDoc))
	require.Error(t, err)
	assert.Equal(t, ErrValidateDocumentCode, meshkiterrors.GetCode(err))

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	require.NotEmpty(t, details.Violations, "missing required fields must produce at least one violation")

	// At least one of the omitted required fields (sub_type, kind, status)
	// must surface as a violation instance path.
	instancePaths := make([]string, 0, len(details.Violations))
	for _, v := range details.Violations {
		instancePaths = append(instancePaths, v.InstancePath)
	}
	foundMissing := false
	for _, missing := range []string{"/sub_type", "/kind", "/status"} {
		for _, p := range instancePaths {
			if p == missing {
				foundMissing = true
			}
		}
	}
	assert.True(t, foundMissing,
		"expected at least one of /sub_type, /kind, /status in violation paths, got: %v", instancePaths)
}

// ---------------------------------------------------------------------------
// Credential documents – no schemaVersion in schema or document
// ---------------------------------------------------------------------------

// syntheticCredentialSchemaYAML is a minimal JSON-Schema-compatible schema
// that mirrors what a credential asset on the meshery/schemas "credentials"
// branch looks like: it defines a typed object with required fields but
// intentionally omits a schemaVersion property entirely.
const syntheticCredentialSchemaYAML = `
$schema: http://json-schema.org/draft-07/schema#
type: object
additionalProperties: true
required:
  - name
  - type
properties:
  name:
    type: string
    description: Human-readable credential name.
  type:
    type: string
    description: Credential type (e.g. kubeconfig, token, basic-auth).
  secret:
    type: object
    description: Opaque map of credential-specific values.
`

// credentialSchemaFS returns a minimal in-memory fs.FS containing a single
// credential schema asset placed in the standard constructs path layout.
// It does not add a schemaVersion property so that tests can rely on the
// exact behaviour produced by the real upstream asset.
func credentialSchemaFS() fstest.MapFS {
	return fstest.MapFS{
		"schemas/constructs/v1beta1/credential/credential.yaml": &fstest.MapFile{
			Data: []byte(syntheticCredentialSchemaYAML),
		},
	}
}

// newCredentialValidator returns a Validator backed by the synthetic credential
// schema FS so that tests are hermetic and do not depend on the released module.
func newCredentialValidator(t *testing.T) *Validator {
	t.Helper()
	v := &Validator{
		fsys:          credentialSchemaFS(),
		registrations: map[string]Registration{},
		typeVersions:  map[DocumentType]map[string]Registration{},
	}
	require.NoError(t, v.Register(Registration{
		Ref:          Ref{Type: DocumentType("credential")},
		Location:     "schemas/constructs/v1beta1/credential/credential.yaml",
		AssetVersion: "v1beta1",
	}))
	return v
}

// TestDiscoverRegistrationCredentialSchemaWithoutSchemaVersion verifies that
// discoverRegistration succeeds for a schema file that has no schemaVersion
// property.  The resulting Registration must carry the inferred document type
// and an empty SchemaVersion – the schema is discoverable, just not
// auto-detectable from a document field.
func TestDiscoverRegistrationCredentialSchemaWithoutSchemaVersion(t *testing.T) {
	fsys := credentialSchemaFS()
	reg, ok, err := discoverRegistration(fsys, "schemas/constructs/v1beta1/credential/credential.yaml")

	require.NoError(t, err)
	require.True(t, ok, "credential schema must be discoverable even without a schemaVersion property")
	assert.Equal(t, DocumentType("credential"), reg.Ref.Type,
		"document type must be inferred from the filename")
	assert.Empty(t, reg.Ref.SchemaVersion,
		"SchemaVersion must be empty when the schema has no schemaVersion property")
	assert.Equal(t, "schemas/constructs/v1beta1/credential/credential.yaml", reg.Location)
	assert.Equal(t, "v1beta1", reg.AssetVersion,
		"AssetVersion must still be derived from the path segment even without schemaVersion")
}

// TestValidatorExplicitTypeCredentialValidationSuccess confirms that a
// credential document can be validated with an explicit Ref{Type: "credential"}
// even though neither the schema nor the document contains a schemaVersion.
// This is the intended usage pattern once a credential schema asset ships.
func TestValidatorExplicitTypeCredentialValidationSuccess(t *testing.T) {
	v := newCredentialValidator(t)

	credentialDoc := `
name: my-k8s-kubeconfig
type: kubeconfig
secret:
  data: ZmFrZS1rdWJlY29uZmlnLWRhdGE=
`
	err := v.ValidateBytes(Ref{Type: DocumentType("credential")}, []byte(credentialDoc))
	require.NoError(t, err,
		"credential document must validate successfully with an explicit Ref{Type}")
}

// TestValidatorExplicitTypeCredentialValidationFailureMissingRequired confirms
// that schema constraints (required fields) are still enforced even when the
// schema has no schemaVersion property.
func TestValidatorExplicitTypeCredentialValidationFailureMissingRequired(t *testing.T) {
	v := newCredentialValidator(t)

	// "type" is required by syntheticCredentialSchemaYAML; omit it.
	incompleteDoc := `
name: my-k8s-kubeconfig
`
	err := v.ValidateBytes(Ref{Type: DocumentType("credential")}, []byte(incompleteDoc))
	require.Error(t, err)
	assert.Equal(t, ErrValidateDocumentCode, meshkiterrors.GetCode(err))

	details, ok := ValidationDetailsFromError(err)
	require.True(t, ok)
	require.NotEmpty(t, details.Violations)
}

// TestValidatorAutoDetectCredentialWithoutSchemaVersionFails documents the
// expected limitation: auto-detection via Validate() requires a schemaVersion
// field in the document.  A credential document that lacks schemaVersion (which
// mirrors the upstream schema's own absence of a schemaVersion property) must
// return ErrDetectSchemaVersion.
func TestValidatorAutoDetectCredentialWithoutSchemaVersionFails(t *testing.T) {
	v := newCredentialValidator(t)

	credentialDoc := `
name: my-k8s-kubeconfig
type: kubeconfig
`
	// Validate() uses auto-detection; without a schemaVersion field it cannot
	// select a schema and must fail with ErrDetectSchemaVersion.
	err := v.Validate([]byte(credentialDoc))
	require.Error(t, err)
	assert.Equal(t, ErrDetectSchemaVersionCode, meshkiterrors.GetCode(err),
		"auto-detection must return ErrDetectSchemaVersion for documents lacking a schemaVersion field")
}

// TestValidatorDocumentTypesIncludesCredentialAfterRegistration verifies that
// the credential type appears in DocumentTypes() once a credential schema is
// registered.  This is the forward-looking check: when the schemas module
// ships a credential asset the type will be auto-discovered, but it can also be
// explicitly registered before that point.
func TestValidatorDocumentTypesIncludesCredentialAfterRegistration(t *testing.T) {
	v := newCredentialValidator(t)

	typeSet := make(map[DocumentType]struct{})
	for _, dt := range v.DocumentTypes() {
		typeSet[dt] = struct{}{}
	}

	assert.Contains(t, typeSet, DocumentType("credential"),
		"DocumentTypes() must include 'credential' after an explicit Register() call")
}

// TestBuiltinRegistrationsDoNotIncludeCredentialCurrently documents the
// current state of the released github.com/meshery/schemas module: no
// credential schema asset has shipped yet, so no credential type appears in
// the default registrations.  This test should be updated (or removed) when
// the credential schema lands in the module.
func TestBuiltinRegistrationsDoNotIncludeCredentialCurrently(t *testing.T) {
	registrations, err := builtinRegistrations()
	require.NoError(t, err)

	for _, reg := range registrations {
		assert.NotEqual(t, DocumentType("credential"), reg.Ref.Type,
			"credential type must not be present in the released schemas module at v0.8.127; "+
				"update this test when the credential schema asset is released")
	}
}
