package v1beta1

import (
	"testing"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta3/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// stubCluster replaces the Helm install and the kind discovery for the duration of a test.
// served maps a component kind to whether the cluster serves it; installs counts Helm calls.
func stubCluster(t *testing.T, served map[string]bool) (installs *[]kubernetes.ApplyHelmChartConfig) {
	t.Helper()
	var calls []kubernetes.ApplyHelmChartConfig
	origApply, origServed := applyHelmChartFunc, kindServed
	t.Cleanup(func() { applyHelmChartFunc, kindServed = origApply, origServed })
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
