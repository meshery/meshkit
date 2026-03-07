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

	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
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
			SchemaVersion: v1beta1.ComponentSchemaVersion,
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
				SchemaVersion: v1beta1.ModelSchemaVersion,
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
	clearDocRefs(doc)
	resolved, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(resolved), nil
}

// clearDocRefs clears $ref strings across the entire OpenAPI document.
func clearDocRefs(doc *openapi3.T) {
	stack := make(map[*openapi3.Schema]bool)
	visited := make(map[*openapi3.PathItem]bool)

	if doc.Components != nil {
		for _, sr := range doc.Components.Schemas {
			clearSchemaRefs(sr, stack)
		}
		for _, pr := range doc.Components.Parameters {
			clearParameterRefs(pr, stack, visited)
		}
		for _, hr := range doc.Components.Headers {
			clearHeaderRefs(hr, stack, visited)
		}
		for _, rb := range doc.Components.RequestBodies {
			clearRequestBodyRefs(rb, stack, visited)
		}
		for _, rr := range doc.Components.Responses {
			clearResponseRefs(rr, stack, visited)
		}
		for _, cr := range doc.Components.Callbacks {
			clearCallbackRefs(cr, stack, visited)
		}
		for _, er := range doc.Components.Examples {
			if er != nil {
				er.Ref = ""
			}
		}
		for _, lr := range doc.Components.Links {
			if lr != nil {
				lr.Ref = ""
			}
		}
		for _, ssr := range doc.Components.SecuritySchemes {
			if ssr != nil {
				ssr.Ref = ""
			}
		}
	}

	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.Map() {
			clearPathItemRefs(pathItem, stack, visited)
		}
	}
}

func clearContentRefs(content openapi3.Content, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	for _, mt := range content {
		if mt != nil {
			clearSchemaRefs(mt.Schema, stack)
		}
	}
}

func clearParameterRefs(pr *openapi3.ParameterRef, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if pr == nil {
		return
	}
	pr.Ref = ""
	if pr.Value != nil {
		clearSchemaRefs(pr.Value.Schema, stack)
		clearContentRefs(pr.Value.Content, stack, visited)
	}
}

func clearHeaderRefs(hr *openapi3.HeaderRef, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if hr == nil {
		return
	}
	hr.Ref = ""
	if hr.Value != nil {
		clearSchemaRefs(hr.Value.Schema, stack)
		clearContentRefs(hr.Value.Content, stack, visited)
	}
}

func clearRequestBodyRefs(rb *openapi3.RequestBodyRef, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if rb == nil {
		return
	}
	rb.Ref = ""
	if rb.Value != nil {
		clearContentRefs(rb.Value.Content, stack, visited)
	}
}

func clearResponseRefs(rr *openapi3.ResponseRef, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if rr == nil {
		return
	}
	rr.Ref = ""
	if rr.Value != nil {
		clearContentRefs(rr.Value.Content, stack, visited)
		for _, hr := range rr.Value.Headers {
			clearHeaderRefs(hr, stack, visited)
		}
	}
}

func clearCallbackRefs(cr *openapi3.CallbackRef, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if cr == nil {
		return
	}
	cr.Ref = ""
	if cr.Value != nil {
		for _, pathItem := range cr.Value.Map() {
			clearPathItemRefs(pathItem, stack, visited)
		}
	}
}

func clearPathItemRefs(pathItem *openapi3.PathItem, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if pathItem == nil || visited[pathItem] {
		return
	}
	visited[pathItem] = true
	for _, pr := range pathItem.Parameters {
		clearParameterRefs(pr, stack, visited)
	}
	for _, op := range pathItem.Operations() {
		clearOperationRefs(op, stack, visited)
	}
}

func clearOperationRefs(op *openapi3.Operation, stack map[*openapi3.Schema]bool, visited map[*openapi3.PathItem]bool) {
	if op == nil {
		return
	}
	for _, pr := range op.Parameters {
		clearParameterRefs(pr, stack, visited)
	}
	clearRequestBodyRefs(op.RequestBody, stack, visited)
	if op.Responses != nil {
		for _, rr := range op.Responses.Map() {
			clearResponseRefs(rr, stack, visited)
		}
	}
	for _, cr := range op.Callbacks {
		clearCallbackRefs(cr, stack, visited)
	}
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
	sr.Ref = ""
	s := sr.Value
	if s == nil {
		return
	}
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
