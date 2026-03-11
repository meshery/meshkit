package schema

import (
	stderrors "errors"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"strings"

	"github.com/dlclark/regexp2"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const embeddedSchemaScheme = "meshkit"

type dlclarkRegexp regexp2.Regexp

var regexpMatchStringErrorf = log.Printf

func (re *dlclarkRegexp) MatchString(value string) bool {
	matched, err := (*regexp2.Regexp)(re).MatchString(value)
	if err != nil {
		regexpMatchStringErrorf("schema: regexp2 MatchString failed for pattern %q: %v", re.String(), err)
	}
	return err == nil && matched
}

func (re *dlclarkRegexp) String() string {
	return (*regexp2.Regexp)(re).String()
}

func compileRegexp(pattern string) (jsonschema.Regexp, error) {
	compiled, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}

	return (*dlclarkRegexp)(compiled), nil
}

type embeddedLoader struct {
	fsys fs.FS
}

func (l embeddedLoader) Load(rawURL string) (any, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if parsedURL.Scheme != embeddedSchemaScheme {
		return nil, fmt.Errorf("unsupported schema URL scheme: %s", parsedURL.Scheme)
	}

	embeddedPath := strings.TrimPrefix(parsedURL.Path, "/")
	if embeddedPath == "" {
		return nil, fmt.Errorf("schema path is empty")
	}

	schemaData, err := fs.ReadFile(l.fsys, embeddedPath)
	if err != nil {
		return nil, err
	}

	document, err := decodeDocument(schemaData)
	if err != nil {
		return nil, err
	}

	rewriteRootID(document, rawURL)

	return document, nil
}

func rewriteRootID(document any, rawURL string) {
	object, ok := document.(map[string]any)
	if !ok {
		return
	}

	if _, hasID := object["$id"]; hasID {
		object["$id"] = stripFragment(rawURL)
	}
}

func stripFragment(rawURL string) string {
	if index := strings.Index(rawURL, "#"); index >= 0 {
		return rawURL[:index]
	}

	return rawURL
}

func (v *Validator) compile(location string) (*jsonschema.Schema, error) {
	if cached, ok := v.cache.Load(location); ok {
		return cached.(*jsonschema.Schema), nil
	}

	compiled, err, _ := v.compiling.Do(location, func() (any, error) {
		if cached, ok := v.cache.Load(location); ok {
			return cached.(*jsonschema.Schema), nil
		}

		compiler := jsonschema.NewCompiler()
		compiler.DefaultDraft(jsonschema.Draft7)
		compiler.UseLoader(embeddedLoader{fsys: v.fsys})
		compiler.UseRegexpEngine(compileRegexp)

		schema, err := compiler.Compile(embeddedSchemaURL(location))
		if err != nil {
			return nil, ErrCompileSchema(location, err)
		}

		actual, _ := v.cache.LoadOrStore(location, schema)
		return actual.(*jsonschema.Schema), nil
	})
	if err != nil {
		return nil, err
	}

	return compiled.(*jsonschema.Schema), nil
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
	var validationErr *jsonschema.ValidationError
	if !stderrors.As(err, &validationErr) {
		return nil
	}

	output := validationErr.BasicOutput()
	if output == nil {
		return nil
	}

	violations := []Violation{}
	collectViolations(output, &violations)
	return violations
}

func collectViolations(output *jsonschema.OutputUnit, violations *[]Violation) {
	if output == nil {
		return
	}

	if output.Error != nil {
		schemaPath := output.AbsoluteKeywordLocation
		if schemaPath == "" {
			schemaPath = output.KeywordLocation
		}

		*violations = append(*violations, Violation{
			InstancePath: output.InstanceLocation,
			SchemaPath:   schemaPath,
			Keyword:      keywordFromLocation(schemaPath),
			Message:      output.Error.String(),
		})
	}

	for index := range output.Errors {
		child := output.Errors[index]
		collectViolations(&child, violations)
	}
}

func keywordFromLocation(location string) string {
	if index := strings.Index(location, "#"); index >= 0 {
		location = location[index+1:]
	}

	location = strings.TrimPrefix(location, "/")
	location = strings.TrimSuffix(location, "/")
	if location == "" {
		return ""
	}

	if index := strings.LastIndex(location, "/"); index >= 0 {
		location = location[index+1:]
	}

	return strings.NewReplacer("~1", "/", "~0", "~").Replace(location)
}
