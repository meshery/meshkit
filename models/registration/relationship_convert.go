package registration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/meshery/schemas/models/v1alpha3/relationship"
	relationshipv1beta3 "github.com/meshery/schemas/models/v1beta3/relationship"
)

// v1beta3StyleKeyRenames lists the metadata.styles keys that v1beta3 spells
// in camelCase and v1alpha3 spells in kebab-case.
var v1beta3StyleKeyRenames = map[string]string{
	"arrowScale":          "arrow-scale",
	"curveStyle":          "curve-style",
	"edgeAnimation":       "edge-animation",
	"fontFamily":          "font-family",
	"fontSize":            "font-size",
	"fontStyle":           "font-style",
	"fontWeight":          "font-weight",
	"lineCap":             "line-cap",
	"lineColor":           "line-color",
	"lineOpacity":         "line-opacity",
	"lineStyle":           "line-style",
	"midTargetArrowColor": "mid-target-arrow-color",
	"midTargetArrowFill":  "mid-target-arrow-fill",
	"midTargetArrowShape": "mid-target-arrow-shape",
	"sourceLabel":         "source-label",
	"targetArrowColor":    "target-arrow-color",
	"targetArrowFill":     "target-arrow-fill",
	"targetArrowShape":    "target-arrow-shape",
	"targetLabel":         "target-label",
	"textOpacity":         "text-opacity",
	"textTransform":       "text-transform",
	"zIndex":              "z-index",
}

const (
	v1beta3MatrixKey  = "matchStrategyMatrix"
	v1alpha3MatrixKey = "match_strategy_matrix"
	stylesPath        = "metadata.styles"
	selectorsPath     = "selectors"
)

// convertRelationshipV1Beta3 converts a v1beta3 relationship definition into
// the v1alpha3 type the registry stores. It never drops data silently: after
// converting, every populated field of the source must be present with an
// equal value in the result, otherwise an error naming the lost fields is
// returned. The top-level "model" is only checked for its name, because
// registration replaces it with the registered model's reference.
func convertRelationshipV1Beta3(in *relationshipv1beta3.RelationshipDefinition) (*relationship.RelationshipDefinition, error) {
	if in == nil {
		return nil, fmt.Errorf("nil v1beta3 relationship definition")
	}

	srcJSON, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal v1beta3 relationship: %w", err)
	}

	var srcMap map[string]any
	if err := json.Unmarshal(srcJSON, &srcMap); err != nil {
		return nil, fmt.Errorf("decode v1beta3 relationship: %w", err)
	}

	mapped, ok := renameKeys(srcMap).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("decode v1beta3 relationship: unexpected document shape")
	}

	mappedJSON, err := json.Marshal(mapped)
	if err != nil {
		return nil, fmt.Errorf("marshal mapped relationship: %w", err)
	}

	var out relationship.RelationshipDefinition
	if err := json.Unmarshal(mappedJSON, &out); err != nil {
		return nil, fmt.Errorf("convert v1beta3 relationship: %w", err)
	}

	dstJSON, err := json.Marshal(&out)
	if err != nil {
		return nil, fmt.Errorf("marshal converted relationship: %w", err)
	}

	var dstMap map[string]any
	if err := json.Unmarshal(dstJSON, &dstMap); err != nil {
		return nil, fmt.Errorf("decode converted relationship: %w", err)
	}

	if lost := findLostFields(mapped, dstMap); len(lost) > 0 {
		return nil, fmt.Errorf("v1beta3 relationship fields cannot be represented losslessly: %s", strings.Join(lost, ", "))
	}

	return &out, nil
}

// renameKeys returns a copy of v with the v1beta3 keys that v1alpha3 spells
// differently renamed. Renaming is limited to the schema-defined places
// (metadata.styles and selectors); free-form maps such as capability
// metadata are left untouched.
func renameKeys(v any) any {
	return renameAt(v, "")
}

func renameAt(v any, path string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			key := k
			if renamed, ok := renamedKey(path, k); ok {
				// Never overwrite a key the document already has.
				if _, clash := t[renamed]; !clash {
					key = renamed
				}
			}
			out[key] = renameAt(val, joinPath(path, k))
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = renameAt(val, path)
		}
		return out
	default:
		return v
	}
}

func renamedKey(path, key string) (string, bool) {
	if path == stylesPath {
		renamed, ok := v1beta3StyleKeyRenames[key]
		return renamed, ok
	}
	if (path == selectorsPath || strings.HasPrefix(path, selectorsPath+".")) && key == v1beta3MatrixKey {
		return v1alpha3MatrixKey, true
	}
	return "", false
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// findLostFields returns the paths of populated fields in src that are
// missing or different in dst.
func findLostFields(src, dst map[string]any) []string {
	var lost []string
	for key, sv := range src {
		dv, ok := dst[key]
		if isEmptyValue(sv) {
			if changedFromEmpty(sv, dv, ok) {
				lost = append(lost, key)
			}
			continue
		}
		if key == "model" {
			// Replaced by the registered model's reference at registration;
			// only the identity (name) must survive.
			sm, sok := sv.(map[string]any)
			dm, dok := dv.(map[string]any)
			if sok && !isEmptyValue(sm["name"]) && (!ok || !dok || !reflect.DeepEqual(sm["name"], dm["name"])) {
				lost = append(lost, "model.name")
			}
			continue
		}
		if !ok {
			lost = append(lost, key)
			continue
		}
		lost = append(lost, diffValue(key, sv, dv)...)
	}
	sort.Strings(lost)
	return lost
}

func diffValue(path string, src, dst any) []string {
	if isEmptyValue(src) {
		return nil
	}
	switch s := src.(type) {
	case map[string]any:
		d, ok := dst.(map[string]any)
		if !ok {
			return []string{path}
		}
		var lost []string
		for k, sv := range s {
			dv, found := d[k]
			if isEmptyValue(sv) {
				if changedFromEmpty(sv, dv, found) {
					lost = append(lost, path+"."+k)
				}
				continue
			}
			if !found {
				lost = append(lost, path+"."+k)
				continue
			}
			lost = append(lost, diffValue(path+"."+k, sv, dv)...)
		}
		return lost
	case []any:
		d, ok := dst.([]any)
		if !ok || len(d) != len(s) {
			return []string{path}
		}
		var lost []string
		for i := range s {
			lost = append(lost, diffValue(fmt.Sprintf("%s[%d]", path, i), s[i], d[i])...)
		}
		return lost
	default:
		if !reflect.DeepEqual(src, dst) {
			return []string{path}
		}
		return nil
	}
}

// changedFromEmpty reports whether an explicit false or 0 in the source
// became a different value in the target. An empty source value may be
// omitted by the target, but it must never turn into another value.
func changedFromEmpty(src, dst any, found bool) bool {
	if !found {
		return false
	}
	switch src.(type) {
	case bool, float64:
		return !reflect.DeepEqual(src, dst)
	}
	return false
}

func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case float64:
		return t == 0
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}
