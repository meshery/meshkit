package kubernetes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	namespaceDoc = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: first\n"
	configMapDoc = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: second\n"

	namedConfigMapDoc = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n  namespace: demo\n"
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
		{"comment-only document", namespaceDoc + "---\n# just a comment\n---\n" + configMapDoc, []string{"Namespace", "ConfigMap"}},
		{"single document", configMapDoc, []string{"ConfigMap"}},
		{
			// A document may start on the separator line itself, and the marker
			// must stay on that line for the decoder to accept it.
			"inline document on the separator line",
			namespaceDoc + "--- {apiVersion: v1, kind: ConfigMap, metadata: {name: second}}\n",
			[]string{"Namespace", "ConfigMap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Chunks holding nothing but a separator are expected; ApplyManifest
			// skips them. Every other chunk has to decode.
			var kinds []string
			for i, manifest := range splitManifests([]byte(tt.contents)) {
				if isEmptyDocument(manifest) {
					continue
				}
				_, obj, err := GetObjectFromManifest(manifest)
				if err != nil {
					t.Fatalf("document %d: GetObjectFromManifest() = %v", i, err)
				}
				kinds = append(kinds, obj.GetKind())
			}
			if strings.Join(kinds, ",") != strings.Join(tt.wantKinds, ",") {
				t.Fatalf("kinds = %v, want %v", kinds, tt.wantKinds)
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

// The error has to name the document that failed, not the chunk the splitter
// produced, so a reader can find it in a multi-document manifest.
func TestApplyManifestNamesTheDocumentThatCannotBeDecoded(t *testing.T) {
	client := newTestAPIClient(t, func(string, string) {})

	contents := strings.Replace(namedConfigMapDoc, "%s", "a", 1) +
		"---\n---\n" +
		"apiVersion: v1\nmetadata:\n  name: b\n"

	err := client.ApplyManifest([]byte(contents), ApplyOptions{})
	if err == nil {
		t.Fatal("ApplyManifest() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "document 2") {
		t.Fatalf("ApplyManifest() = %v, want it to name document 2", err)
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

// newTestAPIClient returns a Client backed by a test API server that answers
// discovery and namespace creation, and reports each ConfigMap create to
// onConfigMapCreate. A real clientset is used because Client.KubeClient is
// concrete; the server keeps the calls off any real cluster.
func newTestAPIClient(t *testing.T, onConfigMapCreate func(method, path string)) *Client {
	t.Helper()

	discovery := func(w http.ResponseWriter, body any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("writing fake discovery response: %v", err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api":
			discovery(w, &metav1.APIVersions{Versions: []string{"v1"}})
		case r.Method == http.MethodGet && r.URL.Path == "/apis":
			discovery(w, &metav1.APIGroupList{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1":
			discovery(w, &metav1.APIResourceList{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
					{Name: "namespaces", Kind: "Namespace"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces/demo/configmaps":
			onConfigMapCreate(r.Method, r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a","namespace":"demo"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	restConfig := rest.Config{Host: server.URL}
	kubeClient, err := kubernetes.NewForConfig(&restConfig)
	if err != nil {
		t.Fatalf("building the clientset: %v", err)
	}

	return &Client{KubeClient: kubeClient, RestConfig: restConfig}
}

// IgnoreErrors must skip only the document it cannot decode. A separator the
// stream reader gives up on would take the valid documents around it with it,
// so both of them have to reach the resource operations.
func TestApplyManifestIgnoreErrorsAppliesDocumentsAroundABadOne(t *testing.T) {
	var (
		mu      sync.Mutex
		applied []string
	)
	client := newTestAPIClient(t, func(method, path string) {
		mu.Lock()
		defer mu.Unlock()
		applied = append(applied, method+" "+path)
	})

	contents := strings.Replace(namedConfigMapDoc, "%s", "a", 1) +
		"---\n--- invalid\n---\n" +
		strings.Replace(namedConfigMapDoc, "%s", "b", 1)

	if err := client.ApplyManifest([]byte(contents), ApplyOptions{IgnoreErrors: true}); err != nil {
		t.Fatalf("ApplyManifest() = %v, want nil when IgnoreErrors is set", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(applied) != 2 {
		t.Fatalf("applied %d documents (%v), want both valid ConfigMaps", len(applied), applied)
	}
	for _, request := range applied {
		if request != "POST /api/v1/namespaces/demo/configmaps" {
			t.Fatalf("unexpected request %q", request)
		}
	}
}
