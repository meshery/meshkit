package validator

import (
	"fmt"
	"testing"

	"github.com/layer5io/meshkit/models/catalog/v1alpha1"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/schemas/models/patterns"
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
			Resource: patterns.PatternFile{
				Name:     "test-design",
				Services: make(map[string]*patterns.Service),
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
			Resource: v1beta1.Model{
				VersionMeta: v1beta1.VersionMeta{
					SchemaVersion: "v1beta1",
					Version:       "1.0.0",
				},
				Category: v1beta1.Category{
					Name: "test",
				},
				Model: v1beta1.ModelEntity{
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
