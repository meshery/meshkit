package expose

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/meshery/meshkit/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	var serviceCreated atomic.Bool
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
			serviceCreated.Store(true)
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
	if !serviceCreated.Load() || len(services) != 1 || services[0].Name != "target-service" {
		t.Fatalf("Expose() created=%v services=%#v", serviceCreated.Load(), services)
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

func TestTraverserVisitContinuesAfterLookupError(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		path     string
		body     string
	}{
		{name: "service", resource: "Service", path: "/api/v1/namespaces/default/services/target", body: `{"apiVersion":"v1","kind":"Service","metadata":{"name":"target","namespace":"default"}}`},
		{name: "pod", resource: "Pod", path: "/api/v1/namespaces/default/pods/target", body: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"target","namespace":"default"},"status":{"phase":"Running"}}`},
		{name: "replication controller", resource: "ReplicationController", path: "/api/v1/namespaces/default/replicationcontrollers/target", body: `{"apiVersion":"v1","kind":"ReplicationController","metadata":{"name":"target","namespace":"default"}}`},
		{name: "deployment", resource: "Deployment", path: "/apis/apps/v1/namespaces/default/deployments/target", body: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"target","namespace":"default"}}`},
		{name: "replica set", resource: "ReplicaSet", path: "/apis/apps/v1/namespaces/default/replicasets/target", body: `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"target","namespace":"default"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && r.URL.Path == tt.path {
					_, _ = w.Write([]byte(tt.body))
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
			log, err := logger.New("test", logger.Options{})
			if err != nil {
				t.Fatalf("logger.New() error = %v", err)
			}
			traverser := Traverser{
				Client: client,
				Logger: log,
				Resources: []Resource{
					{Namespace: "default", Type: tt.resource, Name: "missing"},
					{Namespace: "default", Type: tt.resource, Name: "target"},
				},
			}

			var visited []string
			services, err := traverser.Visit(func(object Object, visitErr error) (*v1.Service, error) {
				if visitErr != nil {
					return nil, visitErr
				}
				visited = append(visited, object.GetName())
				return &v1.Service{}, nil
			}, true)
			if err == nil {
				t.Fatal("Visit() error = nil, want combined lookup error")
			}
			if len(visited) != 1 || visited[0] != "target" {
				t.Errorf("callback visited %v, want only target after the missing resource", visited)
			}
			if len(services) != 1 {
				t.Errorf("Visit() returned %d services, want 1", len(services))
			}
		})
	}
}

func TestTraverserVisitCallbackError(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		path     string
		body     string
	}{
		{name: "service", resource: "Service", path: "/api/v1/namespaces/default/services/", body: `{"apiVersion":"v1","kind":"Service","metadata":{"name":"target","namespace":"default"}}`},
		{name: "pod", resource: "Pod", path: "/api/v1/namespaces/default/pods/", body: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"target","namespace":"default"}}`},
		{name: "replication controller", resource: "ReplicationController", path: "/api/v1/namespaces/default/replicationcontrollers/", body: `{"apiVersion":"v1","kind":"ReplicationController","metadata":{"name":"target","namespace":"default"}}`},
		{name: "deployment", resource: "Deployment", path: "/apis/apps/v1/namespaces/default/deployments/", body: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"target","namespace":"default"}}`},
		{name: "replica set", resource: "ReplicaSet", path: "/apis/apps/v1/namespaces/default/replicasets/", body: `{"apiVersion":"apps/v1","kind":"ReplicaSet","metadata":{"name":"target","namespace":"default"}}`},
	}

	for _, tt := range tests {
		for _, continueOnError := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/continueOnError=%t", tt.name, continueOnError), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, tt.path) {
						name := strings.TrimPrefix(r.URL.Path, tt.path)
						body := strings.Replace(tt.body, `"name":"target"`, `"name":"`+name+`"`, 1)
						_, _ = w.Write([]byte(body))
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
				log, err := logger.New("test", logger.Options{})
				if err != nil {
					t.Fatalf("logger.New() error = %v", err)
				}
				traverser := Traverser{
					Client: client,
					Logger: log,
					Resources: []Resource{
						{Namespace: "default", Type: tt.resource, Name: "target"},
						{Namespace: "default", Type: tt.resource, Name: "later"},
					},
				}

				var visited []string
				services, err := traverser.Visit(func(object Object, _ error) (*v1.Service, error) {
					visited = append(visited, object.GetName())
					if object.GetName() == "target" {
						return &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "from-error"}}, errors.New("callback failed")
					}
					return &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "from-later"}}, nil
				}, continueOnError)

				if err == nil {
					t.Fatal("Visit() error = nil, want callback error")
				}
				wantVisited := []string{"target"}
				var wantServices []string
				if continueOnError {
					wantVisited = append(wantVisited, "later")
					wantServices = append(wantServices, "from-later")
				}
				if !reflect.DeepEqual(visited, wantVisited) {
					t.Errorf("callback visited %v, want %v", visited, wantVisited)
				}
				var gotServices []string
				for _, service := range services {
					gotServices = append(gotServices, service.Name)
				}
				if !reflect.DeepEqual(gotServices, wantServices) {
					t.Errorf("Visit() services %v, want %v", gotServices, wantServices)
				}
			})
		}
	}
}
