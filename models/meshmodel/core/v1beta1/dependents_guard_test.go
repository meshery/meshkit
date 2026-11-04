package v1beta1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta3/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// The handler tests in helpers_test.go stub applyHelmChartFunc to capture the Helm config and to
// inject failures. A deploy now goes through installChartOnceFunc, so by default it hands the
// config to whatever applyHelmChartFunc currently is, and those tests keep checking the same
// things without a cluster. Tests that care about the release lookup replace it with stubHelm.
func init() {
	installChartOnceFunc = func(c *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig, _ func() bool) (kubernetes.HelmInstallOutcome, error) {
		return kubernetes.HelmInstalled, applyHelmChartFunc(c, cfg)
	}
}

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

// fakeHelm stands in for the cluster: a single Helm release that is either deployed or not, the
// kinds the cluster serves without Helm, and a record of every Helm call.
type fakeHelm struct {
	deployed    bool
	servedKinds map[string]bool
	onceErr     error
	onceCalls   int32
	installs    int
	applies     []kubernetes.ApplyHelmChartConfig
}

// stubHelm replaces the install, the upgrade and the kind discovery for the duration of a test.
// served maps a component kind to whether the cluster serves it without a Helm release.
func stubHelm(t *testing.T, served map[string]bool) *fakeHelm {
	t.Helper()
	f := &fakeHelm{servedKinds: served}
	origApply, origOnce, origServed := applyHelmChartFunc, installChartOnceFunc, kindServed
	t.Cleanup(func() { applyHelmChartFunc, installChartOnceFunc, kindServed = origApply, origOnce, origServed })

	applyHelmChartFunc = func(_ *kubernetes.Client, cfg kubernetes.ApplyHelmChartConfig) error {
		f.applies = append(f.applies, cfg)
		return nil
	}
	// Mirrors InstallHelmChartOnce: a deployed release is left alone, then the caller is asked
	// whether the chart's resources already exist, and only then is the chart installed.
	installChartOnceFunc = func(_ *kubernetes.Client, _ kubernetes.ApplyHelmChartConfig, provided func() bool) (kubernetes.HelmInstallOutcome, error) {
		atomic.AddInt32(&f.onceCalls, 1)
		if f.onceErr != nil {
			return kubernetes.HelmInstalled, f.onceErr
		}
		if f.deployed {
			return kubernetes.HelmAlreadyInstalled, nil
		}
		if provided != nil && provided() {
			return kubernetes.HelmProvidedElsewhere, nil
		}
		f.deployed = true
		f.installs++
		return kubernetes.HelmInstalled, nil
	}
	kindServed = func(_ *kubernetes.Client, comp component.ComponentDefinition) bool {
		return f.servedKinds[comp.Component.Kind]
	}
	return f
}

func TestDependentsInstallOncePerCluster(t *testing.T) {
	handlers := map[string]DependencyHandler{
		"artifacthub": ArtifactHub{},
		"github":      GitHub{},
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			helm := stubHelm(t, nil)

			for i, kind := range []string{"InfisicalConnection", "InfisicalAuth", "InfisicalStaticSecret"} {
				summary, err := h.HandleDependents(chartComponent(kind, kind), nil, true, false)
				if err != nil {
					t.Fatalf("component %d (%s): unexpected error %v", i, kind, err)
				}
				if i == 0 && summary != "Deployed dependent helm chart for "+kind {
					t.Fatalf("first component summary = %q, want the install summary", summary)
				}
				if i > 0 && summary != "Dependent helm chart for "+kind+" is already installed" {
					t.Fatalf("component %d summary = %q, want the already-installed summary", i, summary)
				}
			}
			if helm.installs != 1 {
				t.Fatalf("Helm installs = %d, want 1: later components of the model must not install the release again", helm.installs)
			}
		})
	}
}

func TestDependentsSkipWhenKindServedWithoutHelm(t *testing.T) {
	// An operator installed without Helm: there is no release, but the cluster serves the kind.
	helm := stubHelm(t, map[string]bool{"InfisicalAuth": true})

	summary, err := GitHub{}.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, false)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if helm.installs != 0 {
		t.Fatalf("Helm installs = %d, want 0 when the kind is already served", helm.installs)
	}
	if want := "Dependent resources for auth are already available in the cluster"; summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
	}
}

func TestDependentsSkipWhenReleaseDeployed(t *testing.T) {
	handlers := map[string]DependencyHandler{"artifacthub": ArtifactHub{}, "github": GitHub{}}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			// The kind is not served (for example its CRDs were removed), but the release exists.
			helm := stubHelm(t, map[string]bool{"InfisicalAuth": false})
			helm.deployed = true

			summary, err := h.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, false)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if helm.installs != 0 {
				t.Fatalf("Helm installs = %d, want 0 when the release is deployed", helm.installs)
			}
			if want := "Dependent helm chart for auth is already installed"; summary != want {
				t.Fatalf("summary = %q, want %q", summary, want)
			}
		})
	}
}

func TestDependentsReportUnusableRelease(t *testing.T) {
	// A release that exists in a state a new install cannot fix comes back as an error from
	// the install step, and must reach the caller instead of being reported as success.
	wantErr := errors.New(`helm release "operator" in namespace "default" is failed`)
	handlers := map[string]DependencyHandler{"artifacthub": ArtifactHub{}, "github": GitHub{}}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			helm := stubHelm(t, nil)
			helm.onceErr = wantErr

			summary, err := h.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, false)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			if !strings.HasPrefix(summary, "error deploying dependent helm chart for auth") {
				t.Fatalf("summary = %q, want the deploy error summary", summary)
			}
		})
	}
}

func TestDependentsUpgradeRunsHelmUpgrade(t *testing.T) {
	// An explicit upgrade goes straight to ApplyHelmChart, whatever the release or the cluster has.
	helm := stubHelm(t, map[string]bool{"InfisicalAuth": true})
	helm.deployed = true

	summary, err := ArtifactHub{}.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, true)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(helm.applies) != 1 || !helm.applies[0].UpgradeIfInstalled {
		t.Fatalf("expected one ApplyHelmChart call with UpgradeIfInstalled, got %+v", helm.applies)
	}
	if helm.onceCalls != 0 {
		t.Fatalf("install-once calls = %d, want 0 for an explicit upgrade", helm.onceCalls)
	}
	if want := "Deployed dependent helm chart for auth"; summary != want {
		t.Fatalf("summary = %q, want %q", summary, want)
	}
}

func TestDependentsUpgradeFailureIsReported(t *testing.T) {
	stubHelm(t, nil)
	wantErr := errors.New("upgrade failed")
	applyHelmChartFunc = func(*kubernetes.Client, kubernetes.ApplyHelmChartConfig) error { return wantErr }

	summary, err := GitHub{}.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, true, true)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if !strings.HasPrefix(summary, "error deploying dependent helm chart for auth") {
		t.Fatalf("summary = %q, want the deploy error summary", summary)
	}
}

func TestDependentsUndeployNeverCallsHelm(t *testing.T) {
	handlers := map[string]DependencyHandler{"artifacthub": ArtifactHub{}, "github": GitHub{}}
	for name, h := range handlers {
		for _, servedNow := range []bool{true, false} {
			helm := stubHelm(t, map[string]bool{"InfisicalAuth": servedNow})
			summary, err := h.HandleDependents(chartComponent("auth", "InfisicalAuth"), nil, false, false)
			if err != nil {
				t.Fatalf("%s: unexpected error %v", name, err)
			}
			if len(helm.applies) != 0 || helm.onceCalls != 0 {
				t.Fatalf("%s: undeploy called Helm (apply %d, install-once %d), want none", name, len(helm.applies), helm.onceCalls)
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
