package schema

import (
	stderrors "errors"
	"io/fs"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	embeddedSchemaScheme      = "meshkit"
	rootSchemaComponentName   = "Root"
	syntheticOpenAPIVersion   = "3.0.3"
	syntheticOpenAPITitle     = "MeshKit Schema Validator"
	syntheticOpenAPIDocSchema = "1.0.0"

	// defaultRegexpMatchTimeout caps regexp2 backtracking evaluation to guard
	// against potential CPU exhaustion when matching untrusted input against
	// complex patterns. regexp2 uses a backtracking engine (unlike Go's RE2),
	// so unbounded matching can become a denial-of-service vector.
	defaultRegexpMatchTimeout = 2 * time.Second
)

type dlclarkRegexp regexp2.Regexp

// RegexpMatchStringErrorf is called when regexp2 pattern matching fails at
// runtime (e.g. due to a match timeout). It defaults to log.Printf. Library
// consumers may replace it with their own structured logger before the
// Validator is used, for example:
//
//	schema.RegexpMatchStringErrorf = myLogger.Errorf
var RegexpMatchStringErrorf = log.Printf

func (re *dlclarkRegexp) MatchString(value string) bool {
	matched, err := (*regexp2.Regexp)(re).MatchString(value)
	if err != nil {
		RegexpMatchStringErrorf("schema: regexp2 MatchString failed for pattern %q: %v", re.String(), err)
	}
	return err == nil && matched
}

func (re *dlclarkRegexp) String() string {
	return (*regexp2.Regexp)(re).String()
}

func compileRegexp(pattern string) (openapi3.RegexMatcher, error) {
	compiled, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}

	compiled.MatchTimeout = defaultRegexpMatchTimeout
	return (*dlclarkRegexp)(compiled), nil
}

func embeddedSchemaReadURI(fsys fs.FS) openapi3.ReadFromURIFunc {
	return func(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
		if location.Scheme != embeddedSchemaScheme {
			return nil, openapi3.ErrURINotSupported
		}

		embeddedPath := strings.TrimPrefix(location.Path, "/")
		if embeddedPath == "" {
			return nil, stderrors.New("schema path is empty")
		}

		return fs.ReadFile(fsys, embeddedPath)
	}
}

func (v *Validator) compile(location string) (*openapi3.Schema, error) {
	if cached, ok := v.cache.Load(location); ok {
		return cached.(*openapi3.Schema), nil
	}

	compiled, err, _ := v.compiling.Do(location, func() (any, error) {
		if cached, ok := v.cache.Load(location); ok {
			return cached.(*openapi3.Schema), nil
		}

		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		loader.ReadFromURIFunc = embeddedSchemaReadURI(v.fsys)

		document := syntheticOpenAPIDocument(location)
		if err := loader.ResolveRefsIn(document, nil); err != nil {
			return nil, ErrCompileSchema(location, err)
		}

		schema := document.Components.Schemas[rootSchemaComponentName].Value
		if schema == nil {
			return nil, ErrCompileSchema(location, stderrors.New("resolved schema is empty"))
		}

		actual, _ := v.cache.LoadOrStore(location, schema)
		return actual.(*openapi3.Schema), nil
	})
	if err != nil {
		return nil, err
	}

	return compiled.(*openapi3.Schema), nil
}

func syntheticOpenAPIDocument(location string) *openapi3.T {
	return &openapi3.T{
		OpenAPI: syntheticOpenAPIVersion,
		Info: &openapi3.Info{
			Title:   syntheticOpenAPITitle,
			Version: syntheticOpenAPIDocSchema,
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				rootSchemaComponentName: openapi3.NewSchemaRef(embeddedSchemaURL(location), nil),
			},
		},
	}
}

func embeddedSchemaURL(location string) string {
	path, fragment, hasFragment := strings.Cut(location, "#")
	base := embeddedSchemaScheme + ":///" + strings.TrimPrefix(path, "/")
	if !hasFragment || fragment == "" {
		return base
	}

	return base + "#" + fragment
}

func violationsFromError(err error) []Violation {
	if err == nil {
		return nil
	}

	violations := []Violation{}
	collectViolations(err, &violations)
	return violations
}

func collectViolations(err error, violations *[]Violation) bool {
	if err == nil {
		return false
	}

	switch actual := err.(type) {
	case openapi3.MultiError:
		collected := false
		for _, child := range actual {
			if collectViolations(child, violations) {
				collected = true
			}
		}
		return collected
	case *openapi3.SchemaError:
		if collectViolations(actual.Origin, violations) {
			return true
		}

		*violations = append(*violations, Violation{
			InstancePath: jsonPointer(actual.JSONPointer()),
			SchemaPath:   schemaPathFromSchemaError(actual),
			Keyword:      actual.SchemaField,
			Message:      schemaErrorMessage(actual),
		})
		return true
	default:
		return collectViolations(stderrors.Unwrap(err), violations)
	}
}

func schemaPathFromSchemaError(err *openapi3.SchemaError) string {
	if err == nil || err.SchemaField == "" {
		return ""
	}

	return "/" + escapeJSONPointerToken(err.SchemaField)
}

func schemaErrorMessage(err *openapi3.SchemaError) string {
	if err == nil {
		return ""
	}
	if err.Reason != "" {
		return err.Reason
	}
	return err.Error()
}

func jsonPointer(path []string) string {
	if len(path) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, token := range path {
		builder.WriteByte('/')
		builder.WriteString(escapeJSONPointerToken(token))
	}

	return builder.String()
}

func escapeJSONPointerToken(token string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(token)
}

// KeywordFromLocation extracts and unescapes the last JSON Pointer segment
// from a schema location string (e.g. "#/properties/foo~1bar" → "foo/bar").
// Returns an empty string when the location has no meaningful path segments.
func KeywordFromLocation(location string) string {
	// Isolate the fragment portion (after '#').
	_, fragment, hasFragment := strings.Cut(location, "#")
	if !hasFragment {
		fragment = location
	}

	// The fragment is a JSON Pointer like "/properties/foo~1bar".
	// Split on "/" and take the last non-empty token.
	parts := strings.Split(fragment, "/")
	last := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			last = parts[i]
			break
		}
	}

	// Unescape JSON Pointer escape sequences (~1 → /, ~0 → ~).
	// Order matters: ~1 must be replaced before ~0.
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(last)
}
