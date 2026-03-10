package relvalidation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	schemas "github.com/meshery/schemas"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
)

const schemaPath = "schemas/constructs/v1alpha3/relationship/relationship_core.yml"

var (
	cachedSchema *openapi3.Schema
	schemaOnce   sync.Once
	schemaErr    error
)

// getRelationshipSchema returns the cached RelationshipDefinition schema,
// loading it from the embedded schemas filesystem on first call.
func getRelationshipSchema() (*openapi3.Schema, error) {
	schemaOnce.Do(func() {
		cachedSchema, schemaErr = loadRelationshipSchema()
	})
	return cachedSchema, schemaErr
}

// loadRelationshipSchema loads and parses the relationship_core.yml OpenAPI doc
// from the embedded schemas FS, resolving all $refs via the same FS.
func loadRelationshipSchema() (*openapi3.Schema, error) {
	data, err := fs.ReadFile(schemas.Schemas, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}

	data = fixSchemaRefs(data)

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(_ *openapi3.Loader, uri *url.URL) ([]byte, error) {
		path := strings.TrimPrefix(uri.Path, "/")
		return fs.ReadFile(schemas.Schemas, path)
	}

	baseURI, _ := url.Parse("file:///" + schemaPath)
	doc, err := loader.LoadFromDataWithPath(data, baseURI)
	if err != nil {
		return nil, fmt.Errorf("loading schema: %w", err)
	}

	ref := doc.Components.Schemas["RelationshipDefinition"]
	if ref == nil {
		return nil, fmt.Errorf("RelationshipDefinition schema not found in document")
	}

	return ref.Value, nil
}

// stripNulls recursively removes nil values from a JSON map so that
// optional pointer fields (which marshal to null) are treated as absent
// rather than triggering "not nullable" schema errors.
func stripNulls(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			stripNulls(val)
		case []interface{}:
			for _, item := range val {
				if sub, ok := item.(map[string]interface{}); ok {
					stripNulls(sub)
				}
			}
		}
	}
}

// Validate performs Tier 1 validation on a relationship definition.
// Schema validation (required fields, enums, types) is handled by kin-openapi
// against the canonical relationship_core.yml. Cross-field semantic checks
// (taxonomy, selector structure, mutatorRef parity) run as plain Go.
func Validate(rel *relationship.RelationshipDefinition) *ValidationResult {
	result := &ValidationResult{}

	jsonData := validateAgainstSchema(rel, result)
	checkCrossFieldRules(rel, jsonData, result)

	return result
}

// ValidateWithModel performs Tier 1 + Tier 2 validation.
// Additionally verifies that selector component kinds exist in the model's component list.
func ValidateWithModel(
	rel *relationship.RelationshipDefinition,
	mdl *model.ModelDefinition,
	components []component.ComponentDefinition,
) *ValidationResult {
	result := Validate(rel)
	checkSelectorKindsAgainstModel(rel, mdl, components, result)
	return result
}

// validateAgainstSchema marshals the relationship to JSON and validates it
// against the RelationshipDefinition schema using kin-openapi.
// Returns the parsed JSON map for reuse by cross-field checks.
func validateAgainstSchema(rel *relationship.RelationshipDefinition, result *ValidationResult) map[string]interface{} {
	schema, err := getRelationshipSchema()
	if err != nil {
		result.addError("", fmt.Sprintf("schema load failed: %v", err))
		return nil
	}

	data, err := json.Marshal(rel)
	if err != nil {
		result.addError("", fmt.Sprintf("marshal failed: %v", err))
		return nil
	}

	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		result.addError("", fmt.Sprintf("unmarshal failed: %v", err))
		return nil
	}

	m, ok := jsonData.(map[string]interface{})
	if !ok {
		return nil
	}

	// Strip null values so optional pointer fields (which marshal to null)
	// are treated as absent rather than triggering "not nullable" errors.
	stripNulls(m)

	if err := schema.VisitJSON(m); err != nil {
		convertSchemaErrors(err, result)
	}

	return m
}

// convertSchemaErrors translates kin-openapi validation errors into ValidationError entries.
func convertSchemaErrors(err error, result *ValidationResult) {
	if multiErr, ok := err.(openapi3.MultiError); ok {
		for _, e := range multiErr {
			addSchemaError(e, result)
		}
		return
	}
	addSchemaError(err, result)
}

func addSchemaError(err error, result *ValidationResult) {
	if schemaErr, ok := err.(*openapi3.SchemaError); ok {
		field := strings.Join(schemaErr.JSONPointer(), ".")
		result.addError(field, schemaErr.Reason)
	} else {
		result.addError("", err.Error())
	}
}

// --- Cross-field checks (not expressible in the OpenAPI schema) ---

var knownKindTypes = map[relationship.RelationshipDefinitionKind][]string{
	relationship.Edge:         {"binding", "non-binding"},
	relationship.Hierarchical: {"parent"},
	relationship.Sibling:      {"sibling"},
}

var knownTypeSubTypes = map[string][]string{
	"binding":     {"firewall", "mount", "permission", "secret"},
	"non-binding": {"network", "reference", "wallet", "annotation"},
	"parent":      {"inventory", "wallet"},
	"sibling":     {"matchlabels"},
}

func checkCrossFieldRules(rel *relationship.RelationshipDefinition, jsonData map[string]interface{}, result *ValidationResult) {
	checkSelectorsNonEmpty(jsonData, result)
	checkTaxonomy(rel, result)
}

// checkSelectorsNonEmpty validates selector structure using the JSON representation
// to avoid depending on generated Go union types.
func checkSelectorsNonEmpty(jsonData map[string]interface{}, result *ValidationResult) {
	if jsonData == nil {
		return
	}

	selectorsRaw, ok := jsonData["selectors"]
	if !ok || selectorsRaw == nil {
		result.addError("selectors", "must contain at least one selector set")
		return
	}

	selectors, ok := selectorsRaw.([]interface{})
	if !ok || len(selectors) == 0 {
		result.addError("selectors", "must contain at least one selector set")
		return
	}

	for i, selRaw := range selectors {
		sel, ok := selRaw.(map[string]interface{})
		if !ok {
			continue
		}
		prefix := fmt.Sprintf("selectors[%d]", i)

		allowRaw, _ := sel["allow"].(map[string]interface{})
		if allowRaw == nil {
			continue
		}

		fromArr, _ := allowRaw["from"].([]interface{})
		if len(fromArr) == 0 {
			result.addError(prefix+".allow.from", "must contain at least one entry")
		}
		toArr, _ := allowRaw["to"].([]interface{})
		if len(toArr) == 0 {
			result.addError(prefix+".allow.to", "must contain at least one entry")
		}

		for j, itemRaw := range fromArr {
			checkPatchJSON(itemRaw, fmt.Sprintf("%s.allow.from[%d]", prefix, j), result)
		}
		for j, itemRaw := range toArr {
			checkPatchJSON(itemRaw, fmt.Sprintf("%s.allow.to[%d]", prefix, j), result)
		}
	}
}

// checkPatchJSON validates mutatorRef/mutatedRef length parity on the JSON map.
func checkPatchJSON(itemRaw interface{}, prefix string, result *ValidationResult) {
	item, ok := itemRaw.(map[string]interface{})
	if !ok {
		return
	}

	patchRaw, ok := item["patch"].(map[string]interface{})
	if !ok {
		return
	}

	mutatorRef, _ := patchRaw["mutatorRef"].([]interface{})
	mutatedRef, _ := patchRaw["mutatedRef"].([]interface{})

	if len(mutatorRef) > 0 && len(mutatedRef) > 0 && len(mutatorRef) != len(mutatedRef) {
		result.addError(prefix+".patch", fmt.Sprintf(
			"mutatorRef length (%d) does not match mutatedRef length (%d)",
			len(mutatorRef), len(mutatedRef),
		))
	}
}

func checkTaxonomy(rel *relationship.RelationshipDefinition, result *ValidationResult) {
	if rel.Kind == "" {
		return
	}

	knownTypes, ok := knownKindTypes[rel.Kind]
	if !ok {
		return
	}

	if rel.RelationshipType != "" {
		found := false
		for _, t := range knownTypes {
			if strings.EqualFold(rel.RelationshipType, t) {
				found = true
				break
			}
		}
		if !found {
			result.addWarning("type", fmt.Sprintf(
				"%q is not a recognized type for kind %q (known: %s)",
				rel.RelationshipType, rel.Kind, strings.Join(knownTypes, ", "),
			))
		}
	}

	if rel.SubType != "" && rel.RelationshipType != "" {
		knownSubs, ok := knownTypeSubTypes[strings.ToLower(rel.RelationshipType)]
		if ok {
			found := false
			for _, s := range knownSubs {
				if strings.EqualFold(rel.SubType, s) {
					found = true
					break
				}
			}
			if !found {
				result.addWarning("subType", fmt.Sprintf(
					"%q is not a recognized subType for type %q (known: %s)",
					rel.SubType, rel.RelationshipType, strings.Join(knownSubs, ", "),
				))
			}
		}
	}
}

func checkSelectorKindsAgainstModel(
	rel *relationship.RelationshipDefinition,
	mdl *model.ModelDefinition,
	components []component.ComponentDefinition,
	result *ValidationResult,
) {
	if rel.Selectors == nil {
		return
	}

	compKinds := make(map[string]bool, len(components))
	for i := range components {
		compKinds[components[i].Component.Kind] = true
	}

	modelName := mdl.Name

	for i, sel := range *rel.Selectors {
		prefix := fmt.Sprintf("selectors[%d]", i)

		for j, item := range sel.Allow.From {
			checkSelectorKind(item.Kind, item.Model, modelName, compKinds,
				fmt.Sprintf("%s.allow.from[%d].kind", prefix, j), result)
		}
		for j, item := range sel.Allow.To {
			checkSelectorKind(item.Kind, item.Model, modelName, compKinds,
				fmt.Sprintf("%s.allow.to[%d].kind", prefix, j), result)
		}
	}
}

func checkSelectorKind(
	kind *string,
	selectorModel *model.ModelReference,
	currentModelName string,
	compKinds map[string]bool,
	field string,
	result *ValidationResult,
) {
	if kind == nil || *kind == "" || *kind == "*" {
		return
	}

	if selectorModel == nil || selectorModel.Name != currentModelName {
		return
	}

	if !compKinds[*kind] {
		available := make([]string, 0, len(compKinds))
		for k := range compKinds {
			available = append(available, k)
		}
		result.addError(field, fmt.Sprintf(
			"component kind %q not found in model %q (available: %s)",
			*kind, currentModelName, strings.Join(available, ", "),
		))
	}
}
