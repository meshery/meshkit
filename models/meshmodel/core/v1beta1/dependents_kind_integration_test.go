//go:build integration

package v1beta1

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/utils/kubernetes"
	"github.com/meshery/schemas/models/v1beta3/component"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	guardChart     = "https://dl.cloudsmith.io/public/infisical/helm-charts/helm/charts/secrets-operator-0.11.12.tgz"
	guardNamespace = "infisical-guard-test"
	guardRelease   = "secrets-operator"
)

// helmStatus asks the helm CLI, independently of the code under test, for the status of the
// operator release. It returns "" when the release does not exist.
func helmStatus(t *testing.T, kubeconfig string) string {
	t.Helper()
	out, err := exec.Command("helm", "status", guardRelease, "-n", guardNamespace, "--kubeconfig", kubeconfig, "-o", "json").CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "not found") {
			return ""
		}
		t.Fatalf("helm status: %v: %s", err, out)
	}
	var st struct {
		Info struct {
			Status string `json:"status"`
		} `json:"info"`
	}
	if err := json.Unmarshal(out, &st); err != nil {
		t.Fatalf("parsing helm status: %v: %s", err, out)
	}
	return st.Info.Status
}

// Runs the dependents handlers against a real throwaway kind cluster with real Helm and the
// Infisical operator chart. It refuses to run against anything that is not a local kind cluster.
// It needs the helm CLI on the PATH for the failed-release step and as an independent check.
//
//	GUARD_KIND_KUBECONFIG=/path/to/kind.kubeconfig go test -tags integration -run TestGuardOnKind -v ./models/meshmodel/core/v1beta1/
func TestGuardOnKind(t *testing.T) {
	path := os.Getenv("GUARD_KIND_KUBECONFIG")
	if path == "" {
		t.Skip("GUARD_KIND_KUBECONFIG not set")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm CLI not found")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading kubeconfig: %v", err)
	}
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		t.Fatalf("parsing kubeconfig: %v", err)
	}
	if !strings.HasPrefix(cfg.CurrentContext, "kind-") {
		t.Fatalf("refusing to run: current context %q is not a kind cluster", cfg.CurrentContext)
	}
	for name, c := range cfg.Clusters {
		if !strings.Contains(c.Server, "127.0.0.1") && !strings.Contains(c.Server, "localhost") {
			t.Fatalf("refusing to run: cluster %q server %q is not local", name, c.Server)
		}
	}
	kc, err := kubernetes.New(raw)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}

	kinds := []string{"InfisicalConnection", "InfisicalAuth", "InfisicalStaticSecret", "InfisicalStaticSecret"}
	design := make([]component.ComponentDefinition, len(kinds))
	for i, k := range kinds {
		comp := chartComponent(k, k)
		comp.Component.Version = "secrets.infisical.com/v1beta1"
		comp.Metadata.AdditionalProperties["source_uri"] = guardChart
		comp.Configuration = map[string]interface{}{"namespace": guardNamespace}
		design[i] = comp
	}
	cfgFor := kubernetes.ApplyHelmChartConfig{URL: guardChart, Namespace: guardNamespace}

	// dependents_guard_test.go routes the install through a stub for the handler unit tests;
	// this test is about the real thing.
	origOnce := installChartOnceFunc
	installChartOnceFunc = (*kubernetes.Client).InstallHelmChartOnce
	t.Cleanup(func() { installChartOnceFunc = origOnce })

	// A previous run's uninstall deletes the CRDs asynchronously; wait for that to finish.
	for i := 0; i < 60 && kindServed(kc, design[0]); i++ {
		time.Sleep(time.Second)
	}
	if st := helmStatus(t, path); st != "" {
		t.Fatalf("start from a clean cluster: release is %q", st)
	}
	// Remove the release at the end so the test can be run again. This is the only uninstall,
	// and it is test cleanup, not the handlers' behaviour.
	uninstall := func() {
		cleanup := cfgFor
		cleanup.Action = kubernetes.UNINSTALL
		_ = kc.ApplyHelmChart(cleanup)
	}
	t.Cleanup(uninstall)

	deploy := func(comp component.ComponentDefinition, isDeploy, upgrade bool) (string, error) {
		return GitHub{}.HandleDependents(comp, kc, isDeploy, upgrade)
	}
	run := func(label string, isDeploy, upgrade bool) []string {
		var summaries []string
		for _, comp := range design {
			summary, err := deploy(comp, isDeploy, upgrade)
			if err != nil {
				t.Fatalf("%s: %s: unexpected error %v (summary %q)", label, comp.Component.Kind, err, summary)
			}
			t.Logf("%s: %-22s -> %s", label, comp.Component.Kind, summary)
			summaries = append(summaries, summary)
		}
		return summaries
	}
	installed := func(summaries []string) int {
		n := 0
		for _, s := range summaries {
			if strings.HasPrefix(s, "Deployed dependent helm chart") {
				n++
			}
		}
		return n
	}

	// 0. A failed release is reported, not installed over. A real failure is made by installing
	// with the helm CLI and a timeout the operator cannot meet.
	out, err := exec.Command("helm", "install", guardRelease, guardChart, "-n", guardNamespace, "--create-namespace",
		"--kubeconfig", path, "--wait", "--timeout", "1s").CombinedOutput()
	if err == nil {
		t.Fatalf("setup: expected the 1s install to fail, it succeeded: %s", out)
	}
	if st := helmStatus(t, path); st != "failed" {
		t.Fatalf("setup: release status = %q, want failed (helm said: %s)", st, out)
	}
	summary, err := deploy(design[0], true, false)
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("failed release: want an error naming the state, got %v (summary %q)", err, summary)
	}
	t.Logf("failed release: error = %v", err)
	if st := helmStatus(t, path); st != "failed" {
		t.Fatalf("failed release: status changed to %q, want it left alone", st)
	}
	if out, err := exec.Command("helm", "uninstall", guardRelease, "-n", guardNamespace, "--kubeconfig", path).CombinedOutput(); err != nil {
		t.Fatalf("clearing the failed release: %v: %s", err, out)
	}
	// The chart ships its CRDs as templates, so the uninstall deletes them; wait for that to
	// finish before installing them again.
	for i := 0; i < 60 && kindServed(kc, design[0]); i++ {
		time.Sleep(time.Second)
	}

	// 1. First deploy of the four components, all at once: exactly one real install.
	var wg sync.WaitGroup
	first := make([]string, len(design))
	errs := make([]error, len(design))
	for i, comp := range design {
		wg.Add(1)
		go func(i int, comp component.ComponentDefinition) {
			defer wg.Done()
			first[i], errs[i] = deploy(comp, true, false)
		}(i, comp)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent deploy: %s: unexpected error %v (summary %q)", design[i].Component.Kind, err, first[i])
		}
		t.Logf("concurrent deploy: %-22s -> %s", design[i].Component.Kind, first[i])
	}
	if installed(first) != 1 {
		t.Fatalf("concurrent deploy: want exactly one install, got %q", first)
	}
	if st := helmStatus(t, path); st != "deployed" {
		t.Fatalf("after first deploy: release status = %q, want deployed", st)
	}
	if !kindServed(kc, design[0]) {
		t.Fatalf("after first deploy: %s is not served, check the CRD group and version", design[0].Component.Kind)
	}

	// 2. Redeploy: nothing is installed again and nothing fails.
	if n := installed(run("redeploy", true, false)); n != 0 {
		t.Fatalf("redeploy installed %d times, want 0", n)
	}

	// 3. Undeploy: the chart and its CRDs stay.
	for _, s := range run("undeploy", false, false) {
		if !strings.HasPrefix(s, "Left the dependent helm chart") {
			t.Fatalf("undeploy summary %q, want the leave-installed message", s)
		}
	}
	if st := helmStatus(t, path); st != "deployed" {
		t.Fatalf("after undeploy: release status = %q, want the release to remain", st)
	}
	if !kindServed(kc, design[0]) {
		t.Fatal("after undeploy: the CRDs are gone, want them to remain")
	}

	// 4. Deploy again after the undeploy: still no second install.
	if n := installed(run("deploy after undeploy", true, false)); n != 0 {
		t.Fatalf("deploy after undeploy installed %d times, want 0", n)
	}

	// 5. An explicit upgrade still runs, once per component, without a collision.
	for _, s := range run("upgrade", true, true) {
		if !strings.HasPrefix(s, "Deployed dependent helm chart") {
			t.Fatalf("upgrade summary %q, want the deployed message", s)
		}
	}
}
