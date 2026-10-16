package kubernetes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestNewPortForwarderValidation(t *testing.T) {
	tests := []struct {
		name    string
		client  *Client
		target  PortForwardTarget
		wantErr string
	}{
		{name: "nil client", wantErr: "nil kubernetes client"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPortForwarder(tt.client, tt.target, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewPortForwarder() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestPortForwarderTargetDescription(t *testing.T) {
	if got := (&PortForwarder{target: PortForwardTarget{PodName: "pod"}}).targetDesc(); got != "pod" {
		t.Errorf("targetDesc() = %q, want pod", got)
	}
	if got := (&PortForwarder{target: PortForwardTarget{PodLabels: map[string]string{"app": "meshery"}}}).targetDesc(); got != "app=meshery" {
		t.Errorf("targetDesc() = %q, want app=meshery", got)
	}
}

func TestPortForwarderResolvePodByLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/pods") || r.URL.Query().Get("labelSelector") != "app=meshery" {
			t.Errorf("unexpected pod-list request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PodList","items":[{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pending","labels":{"app":"meshery"}},"status":{"phase":"Pending"}},{"apiVersion":"v1","kind":"Pod","metadata":{"name":"ready","labels":{"app":"meshery"}},"status":{"phase":"Running"}}]}`))
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	pf := &PortForwarder{client: &Client{KubeClient: client}, target: PortForwardTarget{Namespace: "default", PodLabels: map[string]string{"app": "meshery"}}}
	got, err := pf.resolvePod()
	if err != nil {
		t.Fatalf("resolvePod() error = %v", err)
	}
	if got != "ready" {
		t.Errorf("resolvePod() = %q, want ready", got)
	}
}

func TestPortForwarderRetriesAfterFailedUpgrade(t *testing.T) {
	requests := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests <- struct{}{}
		http.Error(w, "upgrade unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	pf, err := NewPortForwarder(&Client{KubeClient: clientset, RestConfig: rest.Config{Host: server.URL}}, PortForwardTarget{
		Namespace: "default", PodName: "target", RemotePort: 4222,
	}, nil)
	if err != nil {
		t.Fatalf("NewPortForwarder() error = %v", err)
	}
	pf.Start()
	pf.Start() // Start is idempotent.
	for request := 0; request < 2; request++ {
		select {
		case <-requests:
		case <-time.After(5 * time.Second):
			pf.Stop()
			t.Fatalf("timed out waiting for retry %d", request+1)
		}
	}
	pf.Stop()
	select {
	case <-pf.doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("port-forward did not stop after Stop()")
	}
	address := pf.LocalAddr()
	if !strings.HasPrefix(address, "127.0.0.1:") {
		t.Errorf("LocalAddr() = %q, want loopback address", address)
	}
	select {
	case <-requests:
		t.Error("port-forward retried after Stop()")
	default:
	}
}
