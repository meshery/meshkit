package expose

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestRemove(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/default/services/target" {
			deleted = true
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Success","code":200}`))
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
	if err := Remove("target", "default", client); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !deleted {
		t.Error("Remove() did not send service delete request")
	}
}
