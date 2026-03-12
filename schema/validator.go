package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	meshkitencoding "github.com/meshery/meshkit/encoding"
	meshschemas "github.com/meshery/schemas"
	"golang.org/x/sync/singleflight"
)

// DocumentType identifies a Meshery document family.
type DocumentType string

const (
	TypeComponent    DocumentType = "component"
	TypeConnection   DocumentType = "connection"
	TypeDesign       DocumentType = "design"
	TypeEnvironment  DocumentType = "environment"
	TypeModel        DocumentType = "model"
	TypeRelationship DocumentType = "relationship"
)

// Ref identifies which schema should be used to validate a document.
type Ref struct {
	SchemaVersion string       `json:"schemaVersion,omitempty" yaml:"schemaVersion,omitempty"`
	Type          DocumentType `json:"type,omitempty" yaml:"type,omitempty"`
}

// Registration associates a schema reference with an embedded schema location.
type Registration struct {
	Ref      Ref
	Location string
	// AssetVersion is the schema asset version segment derived from the embedded
	// schemas path (for example, "v1beta1"). It is used to resolve schemaVersion
	// strings even when the underlying schema asset does not embed a literal
	// schemaVersion constant/default.
	AssetVersion string
}

// Violation is a field-level validation failure reported by the underlying schema validator.
type Violation struct {
	InstancePath string `json:"instancePath"`
	SchemaPath   string `json:"schemaPath"`
	Keyword      string `json:"keyword"`
	Message      string `json:"message"`
}

// Validator validates Meshery documents against embedded schemas from github.com/meshery/schemas.
type Validator struct {
	fsys          fs.FS
	registrations map[string]Registration
	typeVersions  map[DocumentType]map[string]Registration
	cache         sync.Map
	compiling     singleflight.Group
	mu            sync.RWMutex
}

var (
	defaultValidator     *Validator
	defaultValidatorOnce sync.Once
)

// Default returns the process-wide validator backed by the embedded schemas module.
func Default() *Validator {
	defaultValidatorOnce.Do(func() {
		defaultValidator = MustNew()
	})

	return defaultValidator
}

// New returns a validator preloaded with the built-in Meshery schema registrations.
func New() (*Validator, error) {
	registrations, err := builtinRegistrations()
	if err != nil {
		return nil, err
	}

	v := &Validator{
		fsys:          meshschemas.Schemas,
		registrations: map[string]Registration{},
		typeVersions:  map[DocumentType]map[string]Registration{},
	}

	for _, registration := range registrations {
		if err := v.Register(registration); err != nil {
			return nil, err
		}
	}

	return v, nil
}

// MustNew returns a validator preloaded with the built-in Meshery schema registrations.
// It panics if any builtin registration fails.
func MustNew() *Validator {
	v, err := New()
	if err != nil {
		panic(err)
	}
	return v
}

// DetectRef reads the schemaVersion from the supplied document and returns the corresponding reference.
func DetectRef(data []byte) (Ref, error) {
	return Default().detectRef(data)
}

func (v *Validator) detectRef(data []byte) (Ref, error) {
	document, err := decodeDocument(data)
	if err != nil {
		return Ref{}, ErrDecodeDocument(err)
	}

	return v.detectRefFromDocument(document)
}

// Validate validates the supplied document after auto-detecting the schema from schemaVersion.
func Validate(data []byte) error {
	return Default().Validate(data)
}

// ValidateWithRef validates the supplied document against the explicitly identified schema.
func ValidateWithRef(ref Ref, data []byte) error {
	return Default().ValidateBytes(ref, data)
}

// ValidateAs validates the supplied document against the schema registered for the given document type.
func ValidateAs(documentType DocumentType, data []byte) error {
	return ValidateWithRef(Ref{Type: documentType}, data)
}

// DecodeAndValidate validates the supplied document after auto-detecting its schema and then decodes it into T.
func DecodeAndValidate[T any](data []byte) (T, error) {
	return DecodeAndValidateWithValidator[T](Default(), Ref{}, data)
}

// DecodeAndValidateWithRef validates the supplied document against the explicitly identified schema and then decodes it into T.
func DecodeAndValidateWithRef[T any](ref Ref, data []byte) (T, error) {
	return DecodeAndValidateWithValidator[T](Default(), ref, data)
}

// DecodeAndValidateWithValidator validates the supplied document using the provided validator and then decodes it into T.
func DecodeAndValidateWithValidator[T any](validator *Validator, ref Ref, data []byte) (T, error) {
	var out T

	if err := validator.ValidateBytes(ref, data); err != nil {
		return out, err
	}

	if err := meshkitencoding.Unmarshal(data, &out); err != nil {
		return out, ErrDecodeTypedDocument(err)
	}

	return out, nil
}

// Validate validates the supplied document after auto-detecting the schema from schemaVersion.
// The document bytes are decoded exactly once; the resulting value is reused for both
// schema-version detection and validation, avoiding a redundant unmarshal.
func (v *Validator) Validate(data []byte) error {
	document, err := decodeDocument(data)
	if err != nil {
		return ErrDecodeDocument(err)
	}

	ref, err := v.detectRefFromDocument(document)
	if err != nil {
		return err
	}

	registration, err := v.resolve(ref)
	if err != nil {
		return err
	}

	return v.validateDocument(registration, ref, document)
}

// ValidateBytes validates the supplied document against the explicitly identified schema.
func (v *Validator) ValidateBytes(ref Ref, data []byte) error {
	if ref.IsZero() {
		return v.Validate(data)
	}

	registration, err := v.resolve(ref)
	if err != nil {
		return err
	}

	document, err := decodeDocument(data)
	if err != nil {
		return ErrDecodeDocument(err)
	}

	return v.validateDocument(registration, ref, document)
}

// ValidateAny validates the supplied value against the explicitly identified schema.
func (v *Validator) ValidateAny(ref Ref, value any) error {
	document, err := normalizeDocument(value)
	if err != nil {
		return ErrDecodeDocument(err)
	}

	if ref.IsZero() {
		ref, err = v.detectRefFromDocument(document)
		if err != nil {
			return err
		}
	}

	registration, err := v.resolve(ref)
	if err != nil {
		return err
	}

	return v.validateDocument(registration, ref, document)
}

func (v *Validator) validateDocument(registration Registration, requested Ref, document any) error {
	resolvedRef := mergeRef(registration.Ref, requested)
	if violation, ok := schemaVersionMismatch(resolvedRef.SchemaVersion, document); ok {
		return ErrValidateDocument(ValidationDetails{
			Ref:            resolvedRef,
			SchemaLocation: registration.Location,
			Violations:     []Violation{violation},
		})
	}

	schema, err := v.compile(registration.Location)
	if err != nil {
		return err
	}

	if err := schema.VisitJSON(
		document,
		openapi3.MultiErrors(),
		openapi3.SetSchemaRegexCompiler(compileRegexp),
	); err != nil {
		return ErrValidateDocument(ValidationDetails{
			Ref:            resolvedRef,
			SchemaLocation: registration.Location,
			Violations:     violationsFromError(err),
		})
	}

	return nil
}

func (r Ref) IsZero() bool {
	return r.SchemaVersion == "" && r.Type == ""
}

func mergeRef(base Ref, fallback Ref) Ref {
	if base.SchemaVersion == "" {
		base.SchemaVersion = fallback.SchemaVersion
	}

	if base.Type == "" {
		base.Type = fallback.Type
	}

	return base
}

func schemaVersionMismatch(expected string, document any) (Violation, bool) {
	if expected == "" {
		return Violation{}, false
	}

	object, ok := document.(map[string]any)
	if !ok {
		return Violation{}, false
	}

	actualValue, found := object["schemaVersion"]
	if !found {
		return Violation{}, false
	}

	actual, ok := actualValue.(string)
	if !ok {
		return Violation{
			InstancePath: "/schemaVersion",
			SchemaPath:   "/schemaVersion",
			Keyword:      "type",
			Message:      "schemaVersion must be a string",
		}, true
	}

	if actual == expected {
		return Violation{}, false
	}

	return Violation{
		InstancePath: "/schemaVersion",
		SchemaPath:   "/schemaVersion",
		Keyword:      "const",
		Message:      fmt.Sprintf("schemaVersion must be %q", expected),
	}, true
}

func (v *Validator) documentTypeFromSchemaVersion(schemaVersion string) DocumentType {
	v.mu.RLock()
	defer v.mu.RUnlock()

	registration, ok := v.registrations[schemaVersionKey(schemaVersion)]
	if !ok {
		registration, ok = v.resolveByDerivedSchemaVersion(schemaVersion, "")
		if !ok {
			return ""
		}
	}

	return registration.Ref.Type
}

func (v *Validator) detectRefFromDocument(document any) (Ref, error) {
	object, ok := document.(map[string]any)
	if !ok {
		return Ref{}, ErrDecodeDocument(fmt.Errorf("schemaVersion can only be auto-detected from object documents"))
	}

	schemaVersion, found := object["schemaVersion"]
	if !found {
		return Ref{}, ErrDetectSchemaVersion()
	}

	schemaVersionString, ok := schemaVersion.(string)
	if !ok {
		return Ref{}, ErrDecodeDocument(fmt.Errorf("schemaVersion must be a string"))
	}

	if schemaVersionString == "" {
		return Ref{}, ErrDetectSchemaVersion()
	}

	return Ref{
		SchemaVersion: schemaVersionString,
		Type:          v.documentTypeFromSchemaVersion(schemaVersionString),
	}, nil
}

func decodeDocument(data []byte) (any, error) {
	var document any

	if err := meshkitencoding.Unmarshal(data, &document); err != nil {
		return nil, err
	}

	return normalizeDocument(document)
}

func normalizeDocument(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			normalized, err := normalizeDocument(val)
			if err != nil {
				return nil, err
			}
			out[k] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			normalized, err := normalizeDocument(elem)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case bool, string, nil,
		float32, float64,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		json.Number:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var result any
		if err := dec.Decode(&result); err != nil {
			return nil, err
		}
		return normalizeDocument(result)
	}
}
