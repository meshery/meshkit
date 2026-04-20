package component

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

// clearDocRefs uses reflection to walk the entire OpenAPI document and clear
// all $ref strings so that json.Marshal outputs fully inlined schemas.
// It uses two tracking mechanisms:
//   - visited: permanent set for general pointers to avoid re-processing
//   - schemaStack: path-based set for *Schema pointers to detect circular
//     schema references (add on enter, remove on exit), allowing the same
//     schema to appear in multiple non-circular positions
func clearDocRefs(doc *openapi3.T) {
	visited := make(map[uintptr]bool)
	schemaStack := make(map[uintptr]bool)
	walkAndClearRefs(reflect.ValueOf(doc), visited, schemaStack)
}

var schemaRefType = reflect.TypeOf((*openapi3.SchemaRef)(nil))

func walkAndClearRefs(v reflect.Value, visited map[uintptr]bool, schemaStack map[uintptr]bool) {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}

		// SchemaRef needs path-based cycle detection so shared (non-circular)
		// schemas are fully expanded while true cycles are broken.
		if v.Type() == schemaRefType {
			sr := v.Interface().(*openapi3.SchemaRef)
			sr.Ref = ""
			if sr.Value == nil {
				return
			}
			schemaPtr := reflect.ValueOf(sr.Value).Pointer()
			if schemaStack[schemaPtr] {
				sr.Value = &openapi3.Schema{}
				return
			}
			schemaStack[schemaPtr] = true
			walkAndClearRefs(reflect.ValueOf(sr.Value), visited, schemaStack)
			delete(schemaStack, schemaPtr)
			return
		}

		ptr := v.Pointer()
		if visited[ptr] {
			return
		}
		visited[ptr] = true

		elem := v.Elem()
		if elem.Kind() == reflect.Struct {
			if refField := elem.FieldByName("Ref"); refField.IsValid() && refField.Kind() == reflect.String {
				refField.SetString("")
			}
		}
		walkAndClearRefs(elem, visited, schemaStack)

	case reflect.Struct:
		// Handle types with unexported map fields (Paths, Callback, Responses)
		// accessed via a Map() method.
		if v.CanAddr() {
			if mapMethod := v.Addr().MethodByName("Map"); mapMethod.IsValid() {
				results := mapMethod.Call(nil)
				if len(results) == 1 && results[0].Kind() == reflect.Map {
					walkAndClearRefs(results[0], visited, schemaStack)
				}
			}
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanInterface() {
				walkAndClearRefs(field, visited, schemaStack)
			}
		}

	case reflect.Map:
		for _, key := range v.MapKeys() {
			walkAndClearRefs(v.MapIndex(key), visited, schemaStack)
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walkAndClearRefs(v.Index(i), visited, schemaStack)
		}

	case reflect.Interface:
		if !v.IsNil() {
			walkAndClearRefs(v.Elem(), visited, schemaStack)
		}
	}
}
