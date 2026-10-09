package describe

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meshkitkube "github.com/meshery/meshkit/utils/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestResourceMap(t *testing.T) {
	tests := []struct {
		name         string
		describeType DescribeType
		want         schema.GroupKind
	}{
		{name: "pod", describeType: Pod, want: schema.GroupKind{Group: "", Kind: "Pod"}},
		{name: "deployment", describeType: Deployment, want: schema.GroupKind{Group: "apps", Kind: "Deployment"}},
		{name: "ingress", describeType: Ingress, want: schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"}},
		{name: "role", describeType: Role, want: schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "Role"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceMap[tt.describeType]; got != tt.want {
				t.Errorf("ResourceMap[%v] = %#v, want %#v", tt.describeType, got, tt.want)
			}
		})
	}
}

func TestDescribeUnsupportedType(t *testing.T) {
	_, err := Describe(&meshkitkube.Client{}, DescriberOptions{Type: DescribeType(999)})
	if err == nil {
		t.Fatal("Describe() error = nil, want unsupported resource error")
	}
}

func TestDescribePod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1":
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod","verbs":["get","list"]},{"name":"events","singularName":"event","namespaced":true,"kind":"Event","verbs":["get","list"]}]}`))
		case "/api/v1/namespaces/default/pods/sample":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"sample","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx"}]},"status":{"phase":"Running"}}`))
		case "/api/v1/namespaces/default/events":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"EventList","items":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
		}
	}))
	defer server.Close()

	client := &meshkitkube.Client{RestConfig: rest.Config{Host: server.URL}}
	output, err := Describe(client, DescriberOptions{Type: Pod, Name: "sample", Namespace: "default"})
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if !strings.Contains(output, "sample") || !strings.Contains(output, "nginx") {
		t.Errorf("Describe() output = %q, want pod name and image", output)
	}
}
