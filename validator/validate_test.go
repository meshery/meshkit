package validator

import (
	"fmt"
	"testing"

	"cuelang.org/go/cue"
)

type ValidationCases struct {
	Path     string
	Resource any

	ShouldPass bool
}

func generateValidResource(t *testing.T, schema cue.Value) any {
	var resource map[string]any
	err := schema.Decode(&resource)
	if err != nil {
		t.Fatal(err)
	}
	return resource
}

func TestValidator(t *testing.T) {
	tests := []ValidationCases{
		{
			Path:       "design",
			ShouldPass: true,
		},
		{
			Path: "catalog_data",
			Resource: map[string]any{
				"pattern_caveats": "NA",
				"pattern_info":    "NA",
				"type":            "Deployment",
			},
			ShouldPass: false,
		},
		{
			Path:       "models",
			ShouldPass: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Path, func(_t *testing.T) {
			schema, err := GetSchemaFor(test.Path)
			if err != nil {
				t.Errorf("%v", err)

			}
			var resource any
			if test.Resource != nil {
				resource = test.Resource
			} else {
				resource = generateValidResource(t, schema)
			}

			err = Validate(schema, resource)
			fmt.Println(err)
			if test.ShouldPass && err != nil {
				t.Errorf("test failed for %s, got %t, want %t, error: %v", test.Path, false, test.ShouldPass, err)

			} else if !test.ShouldPass && err == nil {
				t.Errorf("test failed for %s, got %t, want %t error: %v", test.Path, true, !test.ShouldPass, err)
			}

		})
	}
}
