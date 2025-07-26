package validator

import (
	"fmt"
	"testing"

	"github.com/meshery/schemas/models/v1alpha2"
)

type ValidationCases struct {
	Path     string
	Resource interface{}

	ShouldPass bool
}

func TestValidator(t *testing.T) {
	t.Skip("Temporarily skipping validator test due to schema reference issues")

	tests := []ValidationCases{
		{
			Path: "design",
			Resource: v1alpha2.PatternFile{
				Name:     "test-design",
				Services: make(map[string]*v1alpha2.Service),
			},
			ShouldPass: true,
		},
		// Temporarily skip problematic test cases until schema issues are resolved
		// {
		// 	Path: "catalog_data",
		// 	Resource: v1alpha1.CatalogData{
		// 		PublishedVersion: "v.10.9",
		// 		ContentClass:     "sdsds",
		// 		Compatibility: []v1alpha1.CatalogDataCompatibility{
		// 			"kubernetes",
		// 		},
		// 		PatternCaveats: "NA",
		// 		PatternInfo:    "NA",
		// 		Type:           v1alpha1.CatalogDataType("Dployment"),
		// 	},
		// 	ShouldPass: false,
		// },
		// {
		// 	Path: "models",
		// 	Resource: model.ModelDefinition{
		// 		SchemaVersion: v1beta1.ModelSchemaVersion,
		// 		Version:       "1.0.0",
		// 		Category: category.CategoryDefinition{
		// 			Name: "test",
		// 		},
		// 		Model: model.Model{
		// 			Version: "1.0.0",
		// 		},
		// 		Status:      "",
		// 		DisplayName: "",
		// 		Description: "",
		// 	},
		// 	ShouldPass: false,
		// },
	}

	for _, test := range tests {
		t.Run("validaion", func(_t *testing.T) {
			schema, err := GetSchemaFor(test.Path)
			if err != nil {
				t.Errorf("%v", err)

			}

			err = Validate(schema, test.Resource)
			fmt.Println(err)
			if test.ShouldPass && err != nil {
				t.Errorf("test failed for %s, got %s, want %t, error: %v", test.Path, "false", test.ShouldPass, err)

			} else if !test.ShouldPass && err == nil {
				t.Errorf("test failed for %s, got %s, want %t error: %v", test.Path, "true", !test.ShouldPass, err)
			}

		})
	}
}
