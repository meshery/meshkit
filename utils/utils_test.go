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

func TestGetReleaseTagCommitSHAInvalid(t *testing.T) {
	cases := []struct{
        description string
        org string
        repo string
        tag string
        expectedErr string
    }{
        {
            description: "Test cases negative empty tag",
            org:  "layer5labs",
            repo: "meshery-extensions-packages",
			tag: "",
            expectedErr: "empty release tags",
        },
		{
            description: "Test cases negative old tag",
            org:  "layer5labs",
            repo: "meshery-extensions-packages",
			tag: "v1.0.0",
            expectedErr: "release tags not found",
        },
    }
    for _, tt := range cases {
        t.Run(tt.description, func(t *testing.T){
            commitSHA, err := GetReleaseTagCommitSHA(tt.org, tt.repo, tt.tag)
			if err.Error() != tt.expectedErr {
				t.Errorf("expected error %s, but got error %s", err, err.Error())
			}
          
			if commitSHA != "" {
				t.Errorf("expected commitSHA string empty, but got %s", commitSHA)
			}
        })
    }
}

func TestGetReleaseTagCommitSHA(t *testing.T)  {
	commitSHA, err := GetReleaseTagCommitSHA("kelseyhightower", "nocode", "1.0.0")
	if err != nil {
		t.Errorf("expected no error, but got error %s", err)
	}

	expectedSHA := "ed6c73fc16578ec53ea374585df2b965ce9f4a31"
	if commitSHA != "ed6c73fc16578ec53ea374585df2b965ce9f4a31" {
		t.Errorf("expected commitSHA %s, but got %s", commitSHA, expectedSHA)
	}
}
