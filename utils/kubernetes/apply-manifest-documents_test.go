package kubernetes

import (
	"strings"
	"testing"
)

const (
	namespaceDoc = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: first\n"
	configMapDoc = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: second\n"
)

// Only a "\n---\n" separator was recognised. A separator followed by a comment
// or a trailing space, and a separator in a file with CRLF line endings, were
// not split on, and every document after the first was silently dropped.
func TestSplitManifests(t *testing.T) {
	tests := []struct {
		name      string
		contents  string
		wantKinds []string
	}{
		{"plain separator", namespaceDoc + "---\n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"separator with a comment", namespaceDoc + "--- # second\n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"CRLF line endings", strings.ReplaceAll(namespaceDoc+"---\n"+configMapDoc, "\n", "\r\n"), []string{"Namespace", "ConfigMap"}},
		{"separator with trailing space", namespaceDoc + "--- \n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"leading separator", "---\n" + namespaceDoc + "---\n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"consecutive separators", namespaceDoc + "---\n---\n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"single document", configMapDoc, []string{"ConfigMap"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifests, err := splitManifests([]byte(tt.contents))
			if err != nil {
				t.Fatalf("splitManifests() = %v, want nil", err)
			}
			if len(manifests) != len(tt.wantKinds) {
				t.Fatalf("splitManifests() returned %d documents, want %d", len(manifests), len(tt.wantKinds))
			}
			for i, want := range tt.wantKinds {
				_, obj, err := GetObjectFromManifest(manifests[i])
				if err != nil {
					t.Fatalf("document %d: GetObjectFromManifest() = %v", i, err)
				}
				if got := obj.GetKind(); got != want {
					t.Fatalf("document %d kind = %q, want %q", i, got, want)
				}
			}
		})
	}
}

// A document that cannot be decoded used to be dropped whenever nothing had
// been decoded from it yet, so ApplyManifest returned nil for a manifest that
// was never applied.
func TestApplyManifestReportsDocumentsItCannotDecode(t *testing.T) {
	for name, manifest := range map[string]string{
		"broken yaml":  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n  labels: [unclosed\n",
		"missing kind": "apiVersion: v1\nmetadata:\n  name: b\n",
		"tab indent":   "apiVersion: v1\nkind: ConfigMap\nmetadata:\n\tname: b\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := new(Client).ApplyManifest([]byte(manifest), ApplyOptions{}); err == nil {
				t.Fatal("ApplyManifest() = nil, want an error for a document it cannot apply")
			}
		})
	}
}

// A document holding nothing but whitespace, comments or separators decodes to
// nothing, and applying nothing is the correct outcome for it.
func TestApplyManifestSkipsEmptyDocuments(t *testing.T) {
	for _, contents := range []string{"", "  \n\n", "# only a comment\n", "---\n---\n"} {
		if err := new(Client).ApplyManifest([]byte(contents), ApplyOptions{}); err != nil {
			t.Fatalf("ApplyManifest(%q) = %v, want nil for a document with nothing to apply", contents, err)
		}
	}
}

func TestApplyManifestIgnoreErrorsSkipsUndecodableDocuments(t *testing.T) {
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n  labels: [unclosed\n"
	if err := new(Client).ApplyManifest([]byte(manifest), ApplyOptions{IgnoreErrors: true}); err != nil {
		t.Fatalf("ApplyManifest() = %v, want nil when IgnoreErrors is set", err)
	}
}
