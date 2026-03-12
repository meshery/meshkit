package schema

import (
	stderrors "errors"
	"fmt"

	meshkiterrors "github.com/meshery/meshkit/errors"
)

var (
	ErrInvalidRegistrationCode = "meshkit-11320"
	ErrDetectSchemaVersionCode = "meshkit-11321"
	ErrResolveSchemaCode       = "meshkit-11322"
	ErrDecodeDocumentCode      = "meshkit-11323"
	ErrCompileSchemaCode       = "meshkit-11324"
	ErrValidateDocumentCode    = "meshkit-11325"
	ErrDecodeTypedDocumentCode  = "meshkit-11326"
	ErrUnmarshalSchemaAssetCode = "meshkit-11327"

	errValidateDocumentBase = meshkiterrors.New(
		ErrValidateDocumentCode,
		meshkiterrors.Alert,
		[]string{"Meshery document validation failed"},
		[]string{"The supplied document does not conform to the selected Meshery schema"},
		[]string{"The document contains fields or values that violate one or more schema constraints"},
		[]string{"Inspect the returned validation details to identify the invalid fields and values"},
	)
)

// ValidationDetails contains field-level validation failures for a rejected document.
type ValidationDetails struct {
	Ref            Ref         `json:"ref"`
	SchemaLocation string      `json:"schemaLocation"`
	Violations     []Violation `json:"violations,omitempty"`
}

// ValidationDetailsFromError extracts structured validation details from a MeshKit validation error.
func ValidationDetailsFromError(err error) (ValidationDetails, bool) {
	var meshkitError *meshkiterrors.ErrorV2
	if !stderrors.As(err, &meshkitError) {
		return ValidationDetails{}, false
	}

	details, ok := meshkitError.AdditionalInfo.(ValidationDetails)
	if !ok {
		return ValidationDetails{}, false
	}

	return details, true
}

func ErrInvalidRegistration(err error) error {
	return meshkiterrors.New(
		ErrInvalidRegistrationCode,
		meshkiterrors.Alert,
		[]string{"Invalid schema registration"},
		[]string{err.Error()},
		[]string{"The schema registration is missing a schema reference or an embedded schema location"},
		[]string{"Provide a schemaVersion or type and a non-empty embedded schema location"},
	)
}

func ErrDetectSchemaVersion() error {
	return meshkiterrors.New(
		ErrDetectSchemaVersionCode,
		meshkiterrors.Alert,
		[]string{"Unable to detect schemaVersion"},
		[]string{"The supplied document does not declare schemaVersion"},
		[]string{"The validator can only auto-detect schemas for documents that include schemaVersion"},
		[]string{"Provide Ref{Type: ...} explicitly when validating documents without schemaVersion"},
	)
}

func ErrResolveSchema(ref Ref) error {
	return meshkiterrors.New(
		ErrResolveSchemaCode,
		meshkiterrors.Alert,
		[]string{"Unable to resolve schema"},
		[]string{fmt.Sprintf("No schema registration found for schemaVersion %q and type %q", ref.SchemaVersion, ref.Type)},
		[]string{"The supplied schemaVersion is not registered", "The document type was not provided as a fallback"},
		[]string{"Use one of the built-in Meshery schema versions", "Register a custom schema location before validating custom document types"},
	)
}

func ErrDecodeDocument(err error) error {
	return meshkiterrors.New(
		ErrDecodeDocumentCode,
		meshkiterrors.Alert,
		[]string{"Unable to decode document"},
		[]string{err.Error()},
		[]string{"The supplied document is not valid JSON or YAML"},
		[]string{"Ensure that the document is valid JSON or YAML before validating it"},
	)
}

func ErrCompileSchema(location string, err error) error {
	return meshkiterrors.New(
		ErrCompileSchemaCode,
		meshkiterrors.Alert,
		[]string{"Unable to compile schema"},
		[]string{fmt.Sprintf("Failed to compile schema %q", location), err.Error()},
		[]string{"The embedded schema or one of its references could not be resolved or contains unsupported syntax"},
		[]string{"Ensure the schema location is registered correctly", "Verify that the referenced schema files are available in the embedded schemas module"},
	)
}

func ErrValidateDocument(details ValidationDetails) error {
	err := errValidateDocumentBase.ErrorV2(details)
	return &err
}

func ErrDecodeTypedDocument(err error) error {
	return meshkiterrors.New(
		ErrDecodeTypedDocumentCode,
		meshkiterrors.Alert,
		[]string{"Unable to decode validated document"},
		[]string{err.Error()},
		[]string{"The document is valid against the schema but could not be decoded into the requested Go type"},
		[]string{"Ensure the destination Go type matches the target schema model"},
	)
}

func ErrUnmarshalSchemaAsset(assetPath string, err error) error {
	return meshkiterrors.New(
		ErrUnmarshalSchemaAssetCode,
		meshkiterrors.Alert,
		[]string{"Unable to unmarshal embedded schema asset"},
		[]string{fmt.Sprintf("Failed to unmarshal %s: %s", assetPath, err.Error())},
		[]string{"The embedded schema asset is not valid JSON or YAML"},
		[]string{"Ensure the embedded schema asset is valid JSON or YAML"},
	)
}
