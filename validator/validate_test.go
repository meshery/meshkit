package validator

import (
	"fmt"
	"testing"

	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/models/catalog/v1alpha1"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas/models/v1alpha2"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/category"
	"github.com/meshery/schemas/models/v1beta1/model"
)

type ValidationCases struct {
	Path     string
	Resource interface{}

	ShouldPass bool
}

func TestValidator(t *testing.T) {
	tests := []ValidationCases{
		{
			Path: "design",
			Resource: v1alpha2.PatternFile{
				Name:     "test-design",
				Services: make(map[string]*v1alpha2.Service),
			},
			ShouldPass: true,
		},
		{
			Path: "catalog_data",
			Resource: v1alpha1.CatalogData{
				PublishedVersion: "v.10.9",
				ContentClass:     "sdsds",
				Compatibility: []v1alpha1.CatalogDataCompatibility{
					"kubernetes",
				},
				PatternCaveats: "NA",
				PatternInfo:    "NA",
				Type:           v1alpha1.CatalogDataType("Dployment"),
			},
			ShouldPass: false,
		},
		{
			Path: "models",
			Resource: model.ModelDefinition{

				SchemaVersion: v1beta1.ModelSchemaVersion,
				Version:       "1.0.0",

				Category: category.CategoryDefinition{
					Name: "test",
				},
				Model: model.Model{
					Version: "1.0.0",
				},
				Status:      "",
				DisplayName: "",
				Description: "",
			},
			ShouldPass: false,
		},
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

func TestValidateDeduplicatesErrors(t *testing.T) {
	schema, err := utils.JsonSchemaToCue(`{
		"type": "object",
		"properties": {
			"name": {"allOf": [{"type": "string", "minLength": 3}, {"type": "string", "minLength": 3}]}
		}
	}`)
	if err != nil {
		t.Fatalf("failed to build schema: %v", err)
	}

	err = Validate(schema, map[string]interface{}{"name": 1})
	if err == nil {
		t.Fatal("expected validation to fail for a non-string name")
	}

	meshkitErr, ok := errors.Is(err)
	if !ok {
		t.Fatalf("expected a meshkit error, got %T", err)
	}

	duplicate := "name: conflicting values 1 and strings.MinRunes(3) (mismatched types int and string)"
	count := 0
	for _, msg := range meshkitErr.LongDescription {
		if msg == duplicate {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected %q exactly once, got %d times in %q", duplicate, count, meshkitErr.LongDescription)
	}
}
