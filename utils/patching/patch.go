package patch

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/sjson"
)

// Patch represents a single path/value operation
type Patch struct {
	Path  []string    `json:"path"`
	Value interface{} `json:"value"`
}

// ApplyPatches applies multiple patches to a map
func ApplyPatches(data map[string]interface{}, patches []Patch) (map[string]interface{}, error) {
	// Convert the map to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %w", err)
	}

	jsonStr := string(jsonData)

	// Apply each patch
	for _, patch := range patches {
		// Convert the path array to sjson path format
		path := convertPathToSjsonPath(patch.Path)
		// Apply the patch
		jsonStr, err = sjson.Set(jsonStr, path, patch.Value)

		if err != nil {
			return nil, fmt.Errorf("error applying patch: %w", err)
		}
	}

	// Convert back to map
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling result: %w", err)
	}

	return result, nil
}

// sjsonPathSpecials are the characters gjson/sjson path syntax assigns meaning
// to inside a key. Escaping them makes every path segment a literal key.
const sjsonPathSpecials = `.*?|#@`

// convertPathToSjsonPath converts a path array to sjson path format. Each
// segment is a literal key: dotted Kubernetes annotation and label keys such
// as "cert-manager.io/cluster-issuer" or "kubernetes.io/service-name" must
// reach sjson as one key, so path specials inside a segment are escaped before
// segments are joined with the "." separator. A bare join split such keys into
// nested objects and silently corrupted the patched configuration.
func convertPathToSjsonPath(pathArray []string) string {
	escaped := make([]string, len(pathArray))
	for i, segment := range pathArray {
		escaped[i] = escapeSjsonKey(segment)
	}
	return strings.Join(escaped, ".")
}

// escapeSjsonKey backslash-escapes gjson/sjson path specials in a single key.
// The escape character itself is escaped first so pre-existing backslashes in
// a key survive the round trip.
func escapeSjsonKey(key string) string {
	if !strings.ContainsAny(key, sjsonPathSpecials+`\`) {
		return key
	}
	var b strings.Builder
	b.Grow(len(key) + 4)
	for _, r := range key {
		if r == '\\' || strings.ContainsRune(sjsonPathSpecials, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
