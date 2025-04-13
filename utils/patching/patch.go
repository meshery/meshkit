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

// convertPathToSjsonPath converts a path array to sjson path format
func convertPathToSjsonPath(pathArray []string) string {
	return strings.Join(pathArray, ".")
}
