package patch

import (
	"reflect"
	"testing"
)

func TestApplyPatchesPlainSegments(t *testing.T) {
	data := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(1)},
	}
	result, err := ApplyPatches(data, []Patch{
		{Path: []string{"spec", "replicas"}, Value: float64(3)},
		{Path: []string{"spec", "paused"}, Value: true},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	spec := result["spec"].(map[string]interface{})
	if spec["replicas"] != float64(3) || spec["paused"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// A path segment that is itself a dotted key - Kubernetes annotation and label
// keys - must be written as one literal key, not split into nested objects.
func TestApplyPatchesDottedKeySegment(t *testing.T) {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{},
		},
	}
	result, err := ApplyPatches(data, []Patch{
		{Path: []string{"metadata", "annotations", "cert-manager.io/cluster-issuer"}, Value: "letsencrypt-prod"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	annotations := result["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	got, ok := annotations["cert-manager.io/cluster-issuer"]
	if !ok {
		t.Fatalf("dotted key was split instead of written literally: %#v", annotations)
	}
	if got != "letsencrypt-prod" {
		t.Fatalf("unexpected value: %#v", got)
	}
	if _, split := annotations["cert-manager"]; split {
		t.Fatalf("dotted key was additionally written as nested objects: %#v", annotations)
	}
}

func TestApplyPatchesDottedLabelKey(t *testing.T) {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{"app": "web"},
		},
	}
	result, err := ApplyPatches(data, []Patch{
		{Path: []string{"metadata", "labels", "kubernetes.io/service-name"}, Value: "web-svc"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	labels := result["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	want := map[string]interface{}{"app": "web", "kubernetes.io/service-name": "web-svc"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %#v, want %#v", labels, want)
	}
}

func TestEscapeSjsonKey(t *testing.T) {
	cases := map[string]string{
		"name":                           "name",
		"0":                              "0",
		"_":                              "_",
		"cert-manager.io/cluster-issuer": `cert-manager\.io/cluster-issuer`,
		"a*b?c":                          `a\*b\?c`,
		`back\slash`:                     `back\\slash`,
		"pipe|hash#at@":                  `pipe\|hash\#at\@`,
	}
	for in, want := range cases {
		if got := escapeSjsonKey(in); got != want {
			t.Errorf("escapeSjsonKey(%q) = %q, want %q", in, got, want)
		}
	}
}
