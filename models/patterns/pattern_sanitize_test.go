package patterns_test

import (
	"testing"

	"github.com/meshery/meshkit/models/patterns"
	component "github.com/meshery/schemas/models/v1beta2/component"
	pattern "github.com/meshery/schemas/models/v1beta3/design"
)

func makePatternFile(displayName string, config map[string]interface{}) *pattern.PatternFile {
	return &pattern.PatternFile{
		Components: []*component.ComponentDefinition{
			{
				DisplayName:   displayName,
				Configuration: config,
			},
		},
	}
}

func TestSanitizePattern_TrimsDisplayName(t *testing.T) {
	p := makePatternFile("  my-pvc  ", nil)
	patterns.SanitizePattern(p)
	got := p.Components[0].DisplayName
	if got != "my-pvc" {
		t.Errorf("expected DisplayName %q, got %q", "my-pvc", got)
	}
}

func TestSanitizePattern_TrimsStringKeyWithTrailingSpace(t *testing.T) {
	// Reproduces the 'storage ': 2Gi YAML quoting bug.
	p := makePatternFile("pvc", map[string]interface{}{
		"spec": map[string]interface{}{
			"resources": map[string]interface{}{
				"limits": map[string]interface{}{
					"storage ": "2Gi", // trailing space in key
				},
			},
		},
	})
	patterns.SanitizePattern(p)

	limits := p.Components[0].Configuration["spec"].(map[string]interface{})["resources"].(map[string]interface{})["limits"].(map[string]interface{})
	if _, ok := limits["storage "]; ok {
		t.Error("key 'storage ' with trailing space should have been trimmed")
	}
	if v, ok := limits["storage"]; !ok || v != "2Gi" {
		t.Errorf("expected limits[\"storage\"] = \"2Gi\", got %v", v)
	}
}

func TestSanitizePattern_TrimsStringValue(t *testing.T) {
	// Reproduces the 'test-volume ': trailing-space string value quoting bug.
	p := makePatternFile("deployment", map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "test-volume ", // trailing space in value
						},
					},
				},
			},
		},
	})
	patterns.SanitizePattern(p)

	volumes := p.Components[0].Configuration["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["volumes"].([]interface{})
	vol := volumes[0].(map[string]interface{})
	if got := vol["name"]; got != "test-volume" {
		t.Errorf("expected volumes[0].name = \"test-volume\", got %q", got)
	}
}

func TestSanitizePattern_PreservesNonStringLeaves(t *testing.T) {
	// bool, int, float, nil must pass through unchanged.
	p := makePatternFile("comp", map[string]interface{}{
		"replicas":  3,
		"enabled":   true,
		"ratio":     1.5,
		"optionNil": nil,
	})
	patterns.SanitizePattern(p)

	cfg := p.Components[0].Configuration
	if cfg["replicas"] != 3 {
		t.Errorf("replicas changed: %v", cfg["replicas"])
	}
	if cfg["enabled"] != true {
		t.Errorf("enabled changed: %v", cfg["enabled"])
	}
	if cfg["ratio"] != 1.5 {
		t.Errorf("ratio changed: %v", cfg["ratio"])
	}
	if cfg["optionNil"] != nil {
		t.Errorf("optionNil changed: %v", cfg["optionNil"])
	}
}

func TestSanitizePattern_NilConfigurationIsNoop(t *testing.T) {
	p := makePatternFile("comp", nil)
	// Must not panic.
	patterns.SanitizePattern(p)
	if p.Components[0].Configuration != nil {
		t.Errorf("expected nil Configuration to remain nil")
	}
}

func TestSanitizePattern_EmptyPatternIsNoop(t *testing.T) {
	p := &pattern.PatternFile{}
	// Must not panic on empty component list.
	patterns.SanitizePattern(p)
}
