package utils

import (
	"reflect"
	"strings"
	"testing"
)

var testMap1 = map[string]interface{}{
	"group Priority Minimum": "sdff",
	"minimum Priority":       34,
}

var testMap2 = map[string]interface{}{
	"group Priority Minimum": map[string]interface{}{
		"spaced Word": "lorem epsum",
	},
	"minimum Priority": 34,
}

var testMap3 = map[string]interface{}{
	"properties": map[string]interface{}{
		"spec": map[string]interface{}{
			"description": "lorem epsum",
			"properties": map[string]interface{}{
				"ca Bundle": "lorem epsum",
				"group":     "lorem epsum",
				"group Priority Minimum": map[string]interface{}{
					"Need Some Space": "lorem epsum",
				},
				"insecure Skip TLS Verify": "lorem epsum",
				"required":                 []string{"groupPriorityMinimum"},
			},
		},
	},
}

var testMap3ExpectedOutput = map[string]interface{}{
	"properties": map[string]interface{}{
		"spec": map[string]interface{}{
			"description": "lorem epsum",
			"properties": map[string]interface{}{
				"caBundle": "lorem epsum",
				"group":    "lorem epsum",
				"groupPriorityMinimum": map[string]interface{}{
					"NeedSomeSpace": "lorem epsum",
				},
				"insecureSkipTLSVerify": "lorem epsum",
				"required":              []string{"groupPriorityMinimum"},
			},
		},
	},
}

func TestTransformMapKeys(t *testing.T) {
	var tests = []struct {
		input  map[string]interface{}
		trFunc func(string) string
		want   map[string]interface{}
	}{
		{testMap1, func(s string) string { return strings.ReplaceAll(s, " ", "") }, map[string]interface{}{
			"groupPriorityMinimum": "sdff",
			"minimumPriority":      34,
		}},
		{testMap2, func(s string) string { return strings.ReplaceAll(s, " ", "") }, map[string]interface{}{
			"groupPriorityMinimum": map[string]interface{}{"spacedWord": "lorem epsum"},
			"minimumPriority":      34,
		}},
		{testMap3, func(s string) string { return strings.ReplaceAll(s, " ", "") }, testMap3ExpectedOutput},
	}

	for _, tt := range tests {
		t.Run("transformMapKeys", func(t *testing.T) {
			ans := TransformMapKeys(tt.input, tt.trFunc)
			if !reflect.DeepEqual(ans, tt.want) {
				t.Errorf("got %v, want %v", ans, tt.want)
			}
		})
	}
}
