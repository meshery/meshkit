package expose

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meshery/meshkit/logger"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestTraverserVisitPod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pods/target") {
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"target","namespace":"default"},"status":{"phase":"Running"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logger.New("test", logger.Options{})
	traverser := Traverser{Client: client, Logger: log, Resources: []Resource{{Namespace: "default", Type: "Pod", Name: "target"}}}

	var visited bool
	services, err := traverser.Visit(func(object Object, visitErr error) (*v1.Service, error) {
		if visitErr != nil {
			return nil, visitErr
		}
		visited = object.GetName() == "target" && object.GetObjectKind().GroupVersionKind().Kind == "Pod"
		return &v1.Service{}, nil
	}, false)
	if err != nil {
		t.Fatalf("Visit() error = %v", err)
	}
	if !visited || len(services) != 1 {
		t.Fatalf("Visit() visited=%v services=%d", visited, len(services))
	}
}

func TestExposeCreatesServiceForPod(t *testing.T) {
	serviceCreated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api":
			_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1":
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"v1","resources":[{"name":"pods","singularName":"pod","namespaced":true,"kind":"Pod","verbs":["get","list"]},{"name":"services","singularName":"service","namespaced":true,"kind":"Service","verbs":["get","list","create"]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/default/pods/target":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"target","namespace":"default","labels":{"app":"meshery"}},"spec":{"containers":[{"name":"app","ports":[{"containerPort":8080}]}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces/default/services":
			serviceCreated = true
			var service map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
				t.Errorf("decode service request: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(service)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
		}
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logger.New("test", logger.Options{})
	services, err := Expose(client, rest.Config{Host: server.URL}, Config{Name: "target-service", Log: log}, []Resource{{Namespace: "default", Type: "Pod", Name: "target"}})
	if err != nil {
		t.Fatalf("Expose() error = %v", err)
	}
	if !serviceCreated || len(services) != 1 || services[0].Name != "target-service" {
		t.Fatalf("Expose() created=%v services=%#v", serviceCreated, services)
	}
}

func TestTraverserVisitMissingResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logger.New("test", logger.Options{})
	traverser := Traverser{Client: client, Logger: log, Resources: []Resource{{Namespace: "default", Type: "Pod", Name: "missing"}}}
	_, err = traverser.Visit(func(_ Object, err error) (*v1.Service, error) { return nil, err }, false)
	if err == nil {
		t.Error("Visit() error = nil, want missing-resource error")
	}
}
