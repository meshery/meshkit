package component

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueJson "cuelang.org/go/encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/meshery/meshkit/generators/models"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/manifests"

	"gopkg.in/yaml.v3"

	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/meshery/schemas/models/v1beta3"
	"github.com/meshery/schemas/models/v1beta3/component"
)

func GenerateFromOpenAPI(resource string, pkg models.Package) ([]component.ComponentDefinition, error) {
	if resource == "" {
		return nil, nil
	}
	resource, err := getResolvedManifest(resource)
	if err != nil && errors.Is(err, ErrNoSchemasFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cuectx := cuecontext.New()
	cueParsedManExpr, err := cueJson.Extract("", []byte(resource))
	if err != nil {
		return nil, err
	}

	parsedManifest := cuectx.BuildExpr(cueParsedManExpr)
	definitions, err := utils.Lookup(parsedManifest, "components.schemas")

	if err != nil {
		return nil, err
	}

	fields, err := definitions.Fields()
	if err != nil {
		fmt.Printf("%v\n", err)
		return nil, err
	}
	components := make([]component.ComponentDefinition, 0)

	for fields.Next() {
		fieldVal := fields.Value()
		kindCue, err := utils.Lookup(fieldVal, `"x-kubernetes-group-version-kind"[0].kind`)
		if err != nil {
			continue
		}
		kind, err := kindCue.String()
		if err != nil {
			fmt.Printf("%v", err)
			continue
		}

		crd, err := fieldVal.MarshalJSON()
		if err != nil {
			fmt.Printf("%v", err)
			continue
		}
		versionCue, err := utils.Lookup(fieldVal, `"x-kubernetes-group-version-kind"[0].version`)
		if err != nil {
			continue
		}

		groupCue, err := utils.Lookup(fieldVal, `"x-kubernetes-group-version-kind"[0].group`)
		if err != nil {
			continue
		}

		apiVersion, _ := versionCue.String()
		if g, _ := groupCue.String(); g != "" {
			apiVersion = g + "/" + apiVersion
		}
		modified := make(map[string]interface{}) //Remove the given fields which is either not required by End user (like status) or is prefilled by system (like apiVersion, kind and metadata)
		err = json.Unmarshal(crd, &modified)
		if err != nil {
			fmt.Printf("%v", err)
			continue
		}

		modifiedProps, err := UpdateProperties(fieldVal, cue.ParsePath("properties.spec"), apiVersion)
		if err == nil {
			modified = modifiedProps
		}

		DeleteFields(modified)
		crd, err = json.Marshal(modified)
		if err != nil {
			fmt.Printf("%v", err)
			continue
		}

		// Determine if the resource is namespaced
		var isNamespaced bool

		scopeCue, err := utils.Lookup(fieldVal, `"x-kubernetes-resource".scope`)
		if err == nil {
			scope, err := scopeCue.String()
			if err == nil {
				switch scope {
				case "Namespaced":
					isNamespaced = true
				case "Cluster":
					isNamespaced = false
				}
			}
		} else {
			isNamespaced, err = getResourceScope(resource, kind)
			if err != nil {
				isNamespaced = false
			}
		}

		c := component.ComponentDefinition{
			SchemaVersion: v1beta3.ComponentSchemaVersion,
			Format:        component.JSON,
			Component: component.Component{
				Kind:    kind,
				Version: apiVersion,
				Schema:  string(crd),
			},
			DisplayName: manifests.FormatToReadableString(kind),
			Metadata: component.ComponentDefinition_Metadata{
				IsNamespaced: isNamespaced,
			},
			Model: &model.ModelDefinition{
				SchemaVersion: v1beta3.ModelSchemaVersion,
				Model: model.Model{
					Version: pkg.GetVersion(),
				},
				Name:        pkg.GetName(),
				DisplayName: manifests.FormatToReadableString(pkg.GetName()),
				Metadata: &model.ModelDefinition_Metadata{
					AdditionalProperties: map[string]interface{}{
						"source_uri": pkg.GetSourceURL(),
					},
				},
			},
		}

		components = append(components, c)
	}
	return components, nil
}
func getResourceScope(manifest string, kind string) (bool, error) {
	var m map[string]interface{}

	err := yaml.Unmarshal([]byte(manifest), &m)
	if err != nil {
		return false, utils.ErrDecodeYaml(err)
	}

	paths, ok := m["paths"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("paths not found in manifest")
	}

	for path := range paths {
		if strings.Contains(path, "/namespaces/{namespace}/") && strings.Contains(path, strings.ToLower(kind)) {
			return true, nil // Resource is namespaced
		}
	}

	return false, nil // Resource is cluster-scoped
}

func getResolvedManifest(manifest string) (string, error) {
	// Normalize YAML input to JSON.
	var m map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), &m)
	if err != nil {
		return "", utils.ErrDecodeYaml(err)
	}
	byt, err := json.Marshal(m)
	if err != nil {
		return "", utils.ErrMarshal(err)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(byt)
	if err != nil {
		return "", ErrGetSchema(err)
	}
	if doc.Components == nil || len(doc.Components.Schemas) == 0 {
		return "", ErrNoSchemasFound
	}
	stack := make(map[*openapi3.Schema]bool)
	for _, schemaRef := range doc.Components.Schemas {
		clearSchemaRefs(schemaRef, stack)
	}
	clearDocumentSchemaRefs(doc, stack)
	resolved, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(resolved), nil
}

// clearSchemaRefs recursively clears $ref strings on all nested SchemaRefs
// so that json.Marshal outputs fully inlined schemas. The stack set tracks
// Schema values (not SchemaRef pointers) on the current recursion path to
// detect circular references. kin-openapi resolves $refs by creating
// different SchemaRef objects that share the same underlying Schema pointer,
// so tracking by *Schema is necessary to catch all cycles.
func clearSchemaRefs(sr *openapi3.SchemaRef, stack map[*openapi3.Schema]bool) {
	if sr == nil {
		return
	}
	s := sr.Value
	if s == nil {
		// sr.Ref is set but was never resolved to a value. This happens for
		// $refs in locations kin-openapi's loader does not walk during
		// resolution (encoding.headers[].schema is one such spot). Clearing
		// .Ref here without a .Value to put in its place would leave a
		// SchemaRef{Ref: "", Value: nil}, an empty, invalid ref that panics
		// deep inside SchemaRef.MarshalJSON. Leaving the original $ref
		// string in place is honest: it names something real, it is just
		// something this function was not able to resolve.
		return
	}
	sr.Ref = ""
	if stack[s] {
		sr.Value = &openapi3.Schema{}
		return
	}
	stack[s] = true
	for _, child := range s.AllOf {
		clearSchemaRefs(child, stack)
	}
	for _, child := range s.AnyOf {
		clearSchemaRefs(child, stack)
	}
	for _, child := range s.OneOf {
		clearSchemaRefs(child, stack)
	}
	clearSchemaRefs(s.Not, stack)
	if s.Items != nil {
		clearSchemaRefs(s.Items, stack)
	}
	for _, prop := range s.Properties {
		clearSchemaRefs(prop, stack)
	}
	if s.AdditionalProperties.Schema != nil {
		clearSchemaRefs(s.AdditionalProperties.Schema, stack)
	}
	delete(stack, s)
}

// clearDocumentSchemaRefs extends clearSchemaRefs to the rest of the
// document: doc.Paths and the non-schema parts of doc.Components
// (parameters, request bodies, responses, headers). getResolvedManifest
// previously only cleared doc.Components.Schemas, which is sufficient for
// a document whose only $refs live there, but an arbitrary OpenAPI document
// (this is fed public, third-party specs from GitHub-hosted registrants,
// not just Meshery's own) can carry $refs inside path parameters, request
// bodies, responses and response headers too, and those would otherwise
// reach json.Marshal unresolved.
//
// Only inline (non-$ref) containers are walked. kin-openapi's *Ref types
// (ParameterRef, RequestBodyRef, ResponseRef, HeaderRef) all marshal to
// {"$ref": ...} and ignore .Value entirely whenever .Ref is non-empty, so
// clearing SchemaRefs nested inside a component reached via a non-empty
// .Ref would have no visible effect on the output, and mutating .Value
// there risks corrupting a Schema shared with every other reference to
// that same component.
func clearDocumentSchemaRefs(doc *openapi3.T, stack map[*openapi3.Schema]bool) {
	if doc.Components != nil {
		for _, pref := range doc.Components.Parameters {
			clearParameterSchemaRefs(pref, stack)
		}
		for _, rbref := range doc.Components.RequestBodies {
			clearRequestBodySchemaRefs(rbref, stack)
		}
		for _, rref := range doc.Components.Responses {
			clearResponseSchemaRefs(rref, stack)
		}
		for _, href := range doc.Components.Headers {
			clearHeaderSchemaRefs(href, stack)
		}
		for _, cbref := range doc.Components.Callbacks {
			clearCallbackSchemaRefs(cbref, stack)
		}
	}
	if doc.Paths == nil {
		return
	}
	for _, pathItem := range doc.Paths.Map() {
		clearPathItemSchemaRefs(pathItem, stack)
	}
}

// clearPathItemSchemaRefs clears the path-level parameters and every
// operation (GET, POST, PUT, etc.) defined on this path item.
func clearPathItemSchemaRefs(pathItem *openapi3.PathItem, stack map[*openapi3.Schema]bool) {
	if pathItem == nil {
		return
	}
	for _, pref := range pathItem.Parameters {
		clearParameterSchemaRefs(pref, stack)
	}
	for _, op := range []*openapi3.Operation{
		pathItem.Connect, pathItem.Delete, pathItem.Get, pathItem.Head,
		pathItem.Options, pathItem.Patch, pathItem.Post, pathItem.Put, pathItem.Trace,
	} {
		clearOperationSchemaRefs(op, stack)
	}
}

// clearOperationSchemaRefs clears an operation's parameters, request body,
// responses, and callbacks.
func clearOperationSchemaRefs(op *openapi3.Operation, stack map[*openapi3.Schema]bool) {
	if op == nil {
		return
	}
	for _, pref := range op.Parameters {
		clearParameterSchemaRefs(pref, stack)
	}
	clearRequestBodySchemaRefs(op.RequestBody, stack)
	if op.Responses != nil {
		for _, rref := range op.Responses.Map() {
			clearResponseSchemaRefs(rref, stack)
		}
	}
	for _, cbref := range op.Callbacks {
		clearCallbackSchemaRefs(cbref, stack)
	}
}

// clearCallbackSchemaRefs walks a callback's path items the same way
// doc.Paths is walked: a Callback is a map of runtime expression to
// PathItem, so the actual operations and their request/response schemas
// live one level down.
func clearCallbackSchemaRefs(cbref *openapi3.CallbackRef, stack map[*openapi3.Schema]bool) {
	if cbref == nil || cbref.Ref != "" || cbref.Value == nil {
		return
	}
	for _, key := range cbref.Value.Keys() {
		clearPathItemSchemaRefs(cbref.Value.Value(key), stack)
	}
}

// clearParameterSchemaRefs clears an inline parameter's schema and, if it
// declares content instead of a bare schema, that content's media types.
// Skipped entirely if pref is itself a $ref, see the package-level note on
// clearDocumentSchemaRefs for why.
func clearParameterSchemaRefs(pref *openapi3.ParameterRef, stack map[*openapi3.Schema]bool) {
	if pref == nil || pref.Ref != "" || pref.Value == nil {
		return
	}
	clearSchemaRefs(pref.Value.Schema, stack)
	clearContentSchemaRefs(pref.Value.Content, stack)
}

// clearRequestBodySchemaRefs clears the schemas in an inline request body's
// content.
func clearRequestBodySchemaRefs(rbref *openapi3.RequestBodyRef, stack map[*openapi3.Schema]bool) {
	if rbref == nil || rbref.Ref != "" || rbref.Value == nil {
		return
	}
	clearContentSchemaRefs(rbref.Value.Content, stack)
}

// clearResponseSchemaRefs clears an inline response's content and its
// response headers.
func clearResponseSchemaRefs(rref *openapi3.ResponseRef, stack map[*openapi3.Schema]bool) {
	if rref == nil || rref.Ref != "" || rref.Value == nil {
		return
	}
	clearContentSchemaRefs(rref.Value.Content, stack)
	for _, href := range rref.Value.Headers {
		clearHeaderSchemaRefs(href, stack)
	}
}

// clearHeaderSchemaRefs clears an inline header's schema and content, the
// same shape as a parameter since Header embeds Parameter.
func clearHeaderSchemaRefs(href *openapi3.HeaderRef, stack map[*openapi3.Schema]bool) {
	if href == nil || href.Ref != "" || href.Value == nil {
		return
	}
	clearSchemaRefs(href.Value.Schema, stack)
	clearContentSchemaRefs(href.Value.Content, stack)
}

// clearContentSchemaRefs clears every media type's schema in content, plus
// any per-part headers declared under a multipart encoding entry.
func clearContentSchemaRefs(content openapi3.Content, stack map[*openapi3.Schema]bool) {
	for _, mediaType := range content {
		if mediaType == nil {
			continue
		}
		clearSchemaRefs(mediaType.Schema, stack)
		// Each multipart/form-data part can declare its own Content-Type and
		// headers via an Encoding entry; those headers are ordinary Header
		// objects and can carry their own schema $refs.
		for _, encoding := range mediaType.Encoding {
			if encoding == nil {
				continue
			}
			for _, href := range encoding.Headers {
				clearHeaderSchemaRefs(href, stack)
			}
		}
	}
}
