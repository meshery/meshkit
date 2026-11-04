package v1beta1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta3/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func chartComponent(name, kind string) component.ComponentDefinition {
	comp := component.ComponentDefinition{
		DisplayName: name,
		Metadata: component.ComponentDefinition_Metadata{
			AdditionalProperties: map[string]interface{}{
				"source_uri": "https://example.com/operator-0.1.0.tgz",
			},
		},
	}
	comp.Component.Kind = kind
	comp.Component.Version = "example.io/v1"
	return comp
}

// stubCluster replaces the Helm install, the release lookup and the kind discovery for the
// duration of a test. No release is deployed until a test says so with stubRelease.
// served maps a component kind to whether the cluster serves it; installs counts Helm calls.
func stubCluster(t *testing.T, served map[string]bool) (installs *[]kubernetes.ApplyHelmChartConfig) {
	t.Helper()
	var calls []kubernetes.ApplyHelmChartConfig
	origApply, origServed, origRelease := applyHelmChartFunc, kindServed, releaseDeployedFunc
	t.Cleanup(func() { applyHelmChartFunc, kindServed, releaseDeployedFunc = origApply, origServed, origRelease })
	releaseDeployedFunc = func(*kubernetes.Client, kubernetes.ApplyHelmChartConfig) (bool, error) { return false, nil }
	applyHelmChartFunc = func(_ *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
		calls = append(calls, cfg)
		// The first install makes every kind of the chart available, like the real cluster.
		for k := range served {
			served[k] = true
		}
		return nil
	}
	kindServed = func(_ *kubernetes.Client, comp component.ComponentDefinition) bool {
		return served[comp.Component.Kind]
	}
	return &calls
}

func TestDependentsInstallOncePerCluster(t *testing.T) {
	handlers := map[string]DependencyHandler{
		"artifacthub": ArtifactHub{},
		"github":      GitHub{},
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			kinds := []string{"InfisicalConnection", "InfisicalAuth", "InfisicalStaticSecret"}
			served := map[string]bool{}
			for _, k := range kinds {
				served[k] = false
			}
			installs := stubCluster(t, served)

			for i, kind := range kinds {
				summary, err := h.HandleDependents(chartComponent(kind, kind), nil, true, false)
				if err != nil {
					t.Fatalf("component %d (%s): unexpected error %v", i, kind, err)
				}
				if i == 0 && summary != "Deployed dependent helm chart for "+kind {
					t.Fatalf("first component summary = %q, want the install summary", summary)
				}
				if i > 0 && summary != "Dependent resources for "+kind+" are already available in the cluster" {
					t.Fatalf("component %d summary = %q, want the already-available summary", i, summary)
				}
			}
			if len(*installs) != 1 {
				t.Fatalf("Helm installs = %d, want 1: later components of the model must not install the release again", len(*installs))
			}
		})
	}
}

func TestDependentsRedeployDoesNotReinstall(t *testing.T) {
	installs := stubCluster(t, map[string]bool{"InfisicalAuth": true})

	summary, err := GitHub{}.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, false)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(*installs) != 0 {
		t.Fatalf("Helm installs = %d, want 0 when the kind is already served", len(*installs))
	}
	if want := "Dependent resources for auth are already available in the cluster"; summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
	}
}

func TestDependentsUpgradeOverridesAlreadyServed(t *testing.T) {
	installs := stubCluster(t, map[string]bool{"InfisicalAuth": true})

	ah := ArtifactHub{}
	if _, err := ah.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, true); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(*installs) != 1 {
		t.Fatalf("Helm calls = %d, want 1: an explicit upgrade must run even when the kind is served", len(*installs))
	}
	if !(*installs)[0].UpgradeIfInstalled {
		t.Fatal("expected UpgradeIfInstalled to be set")
	}
}

func TestDependentsUndeployNeverCallsHelm(t *testing.T) {
	handlers := map[string]DependencyHandler{"artifacthub": ArtifactHub{}, "github": GitHub{}}
	for name, h := range handlers {
		for _, servedNow := range []bool{true, false} {
			installs := stubCluster(t, map[string]bool{"InfisicalAuth": servedNow})
			summary, err := h.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, false, false)
			if err != nil {
				t.Fatalf("%s: unexpected error %v", name, err)
			}
			if len(*installs) != 0 {
				t.Fatalf("%s: undeploy called Helm %d times, want 0", name, len(*installs))
			}
			if want := "Left the dependent helm chart for auth installed, it may be in use by other designs"; summary != want {
				t.Fatalf("%s: summary = %q, want %q", name, summary, want)
			}
		}
	}
}

func TestKindServedFailsOpen(t *testing.T) {
	// Without a cluster client, or without a kind to look for, the answer is "not served", so
	// the install is attempted instead of silently skipped.
	comp := chartComponent("auth", "InfisicalAuth")
	if kindServed(nil, comp) {
		t.Fatal("nil client must report not served")
	}
	if kindServed(&kubernetes.Client{}, comp) {
		t.Fatal("client without a kube client must report not served")
	}
	if kindServed(&kubernetes.Client{}, component.ComponentDefinition{}) {
		t.Fatal("component without a kind must report not served")
	}
}

func TestResourceListServesKind(t *testing.T) {
	list := &metav1.APIResourceList{
		GroupVersion: "example.io/v1",
		APIResources: []metav1.APIResource{{Kind: "InfisicalAuth"}, {Kind: "InfisicalConnection"}},
	}
	if !resourceListServesKind(list, "InfisicalAuth") {
		t.Fatal("expected InfisicalAuth to be served")
	}
	if resourceListServesKind(list, "Missing") {
		t.Fatal("expected Missing not to be served")
	}
}

// stubRelease overrides the release lookup installed by stubCluster and returns a call counter.
func stubRelease(t *testing.T, deployed bool, err error) *int32 {
	t.Helper()
	var calls int32
	releaseDeployedFunc = func(*kubernetes.Client, kubernetes.ApplyHelmChartConfig) (bool, error) {
		atomic.AddInt32(&calls, 1)
		return deployed, err
	}
	return &calls
}

func TestDependentsSkipWhenReleaseDeployed(t *testing.T) {
	handlers := map[string]DependencyHandler{"artifacthub": ArtifactHub{}, "github": GitHub{}}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			// The kind is not served (for example its CRDs were removed), but the release exists.
			installs := stubCluster(t, map[string]bool{"InfisicalAuth": false})
			lookups := stubRelease(t, true, nil)

			summary, err := h.HandleDependents(chartComponent("auth", "InfisicalAuth"), &kubernetes.Client{}, true, false)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(*installs) != 0 {
				t.Fatalf("Helm installs = %d, want 0 when the release is deployed", len(*installs))
			}
			if *lookups != 1 {
				t.Fatalf("release lookups = %d, want 1", *lookups)
			}
			if want := "Dependent helm chart for auth is already installed"; summary != want {
				t.Fatalf("summary = %q, want %q", summary, want)
			}
		})
	}
}

func TestDependentsReleaseLookupFailureFallsThrough(t *testing.T) {
	// A failed lookup must not block the install.
	installs := stubCluster(t, map[string]bool{"InfisicalAuth": false})
	stubRelease(t, false, errors.New("cluster unreachable"))

	if _, err := (GitHub{}).HandleDependents(chartComponent("auth", "InfisicalAuth"), &kubernetes.Client{}, true, false); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(*installs) != 1 {
		t.Fatalf("Helm installs = %d, want 1 when the release lookup fails", len(*installs))
	}
}

func TestDependentsUpgradeSkipsReleaseLookup(t *testing.T) {
	installs := stubCluster(t, map[string]bool{"InfisicalAuth": true})
	lookups := stubRelease(t, true, nil)

	if _, err := (GitHub{}).HandleDependents(chartComponent("auth", "InfisicalAuth"), &kubernetes.Client{}, true, true); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if *lookups != 0 {
		t.Fatalf("release lookups = %d, want 0 for an explicit upgrade", *lookups)
	}
	if len(*installs) != 1 || !(*installs)[0].UpgradeIfInstalled {
		t.Fatalf("expected one Helm call with UpgradeIfInstalled, got %+v", *installs)
	}
}

func TestDependentsUndeployDoesNotLookUpRelease(t *testing.T) {
	stubCluster(t, map[string]bool{"InfisicalAuth": true})
	lookups := stubRelease(t, true, nil)

	if _, err := (ArtifactHub{}).HandleDependents(chartComponent("auth", "InfisicalAuth"), &kubernetes.Client{}, false, false); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if *lookups != 0 {
		t.Fatalf("release lookups = %d, want 0 on undeploy", *lookups)
	}
}

// discoveryClient returns a client whose API discovery is served by a test server, with the
// given group/version paths returning one resource of each listed kind. hits counts requests.
func discoveryClient(t *testing.T, served map[string][]string) (*kubernetes.Client, *int32) {
	t.Helper()
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		kinds, ok := served[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		list := metav1.APIResourceList{GroupVersion: r.URL.Path[len("/apis/"):]}
		for _, k := range kinds {
			list.APIResources = append(list.APIResources, metav1.APIResource{Kind: k})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	}))
	t.Cleanup(ts.Close)

	cs, err := k8s.NewForConfig(&rest.Config{Host: ts.URL})
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	return &kubernetes.Client{KubeClient: cs}, &hits
}

func TestKindServedAgainstDiscovery(t *testing.T) {
	kc, _ := discoveryClient(t, map[string][]string{"/apis/example.io/v1": {"InfisicalAuth"}})

	if !kindServed(kc, chartComponent("auth", "InfisicalAuth")) {
		t.Fatal("expected a CRD kind the cluster serves to count as served")
	}
	if kindServed(kc, chartComponent("other", "Missing")) {
		t.Fatal("expected a kind the group does not list not to count")
	}
	gone := chartComponent("auth", "InfisicalAuth")
	gone.Component.Version = "gone.io/v1"
	if kindServed(kc, gone) {
		t.Fatal("expected an unserved group to count as not served")
	}
}

func TestKindServedIgnoresBuiltInKinds(t *testing.T) {
	// Built-in kinds are always served and say nothing about the operator, so they must not
	// count, and discovery is not even asked.
	kc, hits := discoveryClient(t, map[string][]string{"/apis/apps/v1": {"Deployment"}, "/api/v1": {"Pod"}})

	for _, tc := range []struct{ version, kind string }{{"apps/v1", "Deployment"}, {"v1", "Pod"}} {
		comp := chartComponent("builtin", tc.kind)
		comp.Component.Version = tc.version
		if kindServed(kc, comp) {
			t.Fatalf("%s %s must not count as served", tc.version, tc.kind)
		}
	}
	if *hits != 0 {
		t.Fatalf("discovery requests = %d, want 0 for built-in groups", *hits)
	}
}
