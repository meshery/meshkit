package kubernetes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestRESTServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis/apiextensions.k8s.io/v1/customresourcedefinitions" {
			t.Errorf("request path = %q, want custom resource definitions path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func newKubernetesAPIServer(t *testing.T) *httptest.Server {
	return newKubernetesAPIServerWithConfigMapCreateStatus(t, http.StatusCreated)
}

func newKubernetesAPIServerWithConfigMapCreateStatus(t *testing.T, createStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"major":"1","minor":"30","gitVersion":"v1.30.0","gitCommit":"test","gitTreeState":"clean","buildDate":"test","goVersion":"go1.26","compiler":"gc","platform":"linux/amd64"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api":
			_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1":
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"configmaps","singularName":"configmap","namespaced":true,"kind":"ConfigMap","verbs":["get","list","create","update","patch","delete"]},{"name":"namespaces","singularName":"namespace","namespaced":false,"kind":"Namespace","verbs":["get","list","create","update","patch","delete"]}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"default"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces/default/configmaps":
			w.WriteHeader(createStatus)
			if createStatus == http.StatusConflict {
				_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"AlreadyExists","code":409}`))
				return
			}
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"settings","namespace":"default"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/namespaces/default/configmaps/settings":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"settings","namespace":"default"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/default/configmaps/settings":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Success","code":200}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/default/pods":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"running","namespace":"default","labels":{"app":"meshery"}},"status":{"phase":"Running"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/default/pods/target":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"target","namespace":"default"},"status":{"phase":"Running"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/default/services/target":
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Success","code":200}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
		}
	}))
}
