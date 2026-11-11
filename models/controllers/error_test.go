package controllers

import (
	stderrors "errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/meshery/meshkit/errors"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

// kubeconfigText is the probable cause ErrDeployController used to give for every failure.
const kubeconfigText = "permission to read kubeconfig"

func TestErrDeployControllerReportsTheClassifiedHelmFailure(t *testing.T) {
	cases := []struct {
		name      string
		inner     error
		wantCode  string
		wantCause string // a probable cause that must be given
	}{
		{"release pending", mesherykube.ErrHelmReleasePending("meshery-operator", "meshery", "pending-install", 1, time.Minute, 10*time.Minute, false),
			ErrOperatorHelmReleaseStateCode, "still running"},
		{"release uninstalling", mesherykube.ErrHelmReleaseState("meshery-operator", "meshery", "uninstalling", 2),
			ErrOperatorHelmReleaseStateCode, "interrupted"},
		{"cluster unreachable", mesherykube.ErrHelmClusterUnreachable("meshery-operator", "meshery", stderrors.New("dial tcp: connection refused")),
			ErrOperatorClusterAccessCode, "could not be reached"},
		{"release record forbidden", mesherykube.ErrHelmReleaseStorage("meshery-operator", "meshery", stderrors.New(`secrets is forbidden`)),
			ErrOperatorClusterAccessCode, "Secrets in the meshery namespace"},
		{"chart not fetched", mesherykube.ErrHelmChartFetch(stderrors.New("chart \"meshery-operator\" version \"v9.9.9\" not found")),
			ErrOperatorChartFetchCode, "not published"},
		{"helm action failed", mesherykube.ErrApplyHelmChart(stderrors.New("admission webhook denied the request")),
			ErrDeployControllerCode, "Helm could not install or upgrade"},
		{"not a meshkit error", stderrors.New("something else"),
			ErrDeployControllerCode, "Helm could not install or upgrade"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ErrDeployController(tc.inner)
			if got := errors.GetCode(err); got != tc.wantCode {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
			cause := errors.GetCause(err)
			if !strings.Contains(cause, tc.wantCause) {
				t.Fatalf("probable cause %q should contain %q", cause, tc.wantCause)
			}
			if strings.Contains(err.Error(), kubeconfigText) {
				t.Fatalf("error %q still blames kubeconfig file permissions", err)
			}
			if strings.Contains(err.Error(), "  ") {
				t.Fatalf("error %q contains a double space", err)
			}
			// The instance text of the failure is carried, without the inner error's own cause and
			// remediation, which would sit beside this error's.
			long := errors.GetLDescription(err)
			if !strings.Contains(long, firstLine(tc.inner)) {
				t.Fatalf("long description %q does not carry the failure %q", long, firstLine(tc.inner))
			}
			if strings.Contains(long, "Probable Cause") || strings.Contains(long, "Suggested Remediation") {
				t.Fatalf("long description %q nests the inner error's cause or remediation", long)
			}
			if verr := errors.ValidateCauses(errors.GetCauses(err)); verr != nil {
				t.Fatalf("invalid causes: %v", verr)
			}
		})
	}
}

func TestErrUndeployControllerSaysUndeploy(t *testing.T) {
	err := ErrUndeployController(mesherykube.ErrApplyHelmChart(stderrors.New(`uninstall: Release not loaded: meshery-operator: release: not found`)))
	if got := errors.GetCode(err); got != ErrUndeployControllerCode {
		t.Fatalf("code = %q, want %q", got, ErrUndeployControllerCode)
	}
	if sd := errors.GetSDescription(err); !strings.Contains(sd, "undeploying") {
		t.Fatalf("short description %q should say undeploying", sd)
	}
	if verr := errors.ValidateCauses(errors.GetCauses(err)); verr != nil {
		t.Fatalf("invalid causes: %v", verr)
	}

	// A classified failure is reported by its class, as on deploy.
	err = ErrUndeployController(mesherykube.ErrHelmClusterUnreachable("meshery-operator", "meshery", stderrors.New("connection refused")))
	if got := errors.GetCode(err); got != ErrOperatorClusterAccessCode {
		t.Fatalf("code = %q, want %q", got, ErrOperatorClusterAccessCode)
	}
}

func TestOperatorErrorsAreValid(t *testing.T) {
	for name, err := range map[string]error{
		"ErrOperatorHelmReleaseState":     ErrOperatorHelmReleaseState(stderrors.New("x")),
		"ErrOperatorClusterAccess":        ErrOperatorClusterAccess(stderrors.New("x")),
		"ErrOperatorChartFetch":           ErrOperatorChartFetch(stderrors.New("x")),
		"ErrOperatorDeploymentInProgress": ErrOperatorDeploymentInProgress(),
	} {
		if verr := errors.ValidateCauses(errors.GetCauses(err)); verr != nil {
			t.Errorf("%s: invalid causes: %v", name, verr)
		}
		if strings.Contains(err.Error(), kubeconfigText) {
			t.Errorf("%s: %q blames kubeconfig file permissions", name, err)
		}
	}
}

// operatorDeployment returns the meshery-operator Deployment with the given rollout progress.
func operatorDeployment(updated int32) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "meshery-operator", Namespace: "meshery", Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1, Replicas: 1, UpdatedReplicas: updated, AvailableReplicas: updated,
		},
	}
}

// operatorHandler returns a Meshery Operator handler for a fake cluster holding objects, whose
// API server is at apiServer and whose chart repository is chartRepo.
func operatorHandler(t *testing.T, apiServer, chartRepo string, objects ...runtime.Object) IMesheryController {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("building scheme: %v", err)
	}
	client := &mesherykube.Client{
		RestConfig:        rest.Config{Host: apiServer},
		DynamicKubeClient: dynamicfake.NewSimpleDynamicClient(scheme, objects...),
	}
	return NewMesheryOperatorHandler(client, OperatorDeploymentConfig{
		GetHelmOverrides:      func(bool) map[string]interface{} { return nil },
		HelmChartRepo:         chartRepo,
		MesheryReleaseVersion: "v0.0.1",
	})
}

func TestDeployRefusesWhileTheOperatorRollsOut(t *testing.T) {
	mo := operatorHandler(t, "http://127.0.0.1:1", "http://127.0.0.1:1", operatorDeployment(0))

	err := mo.Deploy(false)
	if got := errors.GetCode(err); got != ErrOperatorDeploymentInProgressCode {
		t.Fatalf("Deploy() = %v (code %q), want %q", err, got, ErrOperatorDeploymentInProgressCode)
	}
	if strings.Contains(err.Error(), kubeconfigText) {
		t.Fatalf("error %q blames kubeconfig file permissions", err)
	}
}

func TestDeployReportsAnUnreachableChartRepository(t *testing.T) {
	repo := httptest.NewServer(http.HandlerFunc(http.NotFound))
	t.Cleanup(repo.Close)
	mo := operatorHandler(t, "http://127.0.0.1:1", repo.URL) // no operator Deployment: NotDeployed

	err := mo.Deploy(false)
	if got := errors.GetCode(err); got != ErrOperatorChartFetchCode {
		t.Fatalf("Deploy() = %v (code %q), want %q", err, got, ErrOperatorChartFetchCode)
	}
}

func TestUndeployReportsAnUnreachableCluster(t *testing.T) {
	// An address nothing listens on: the API server refuses the connection.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving an address: %v", err)
	}
	apiServer := "http://" + l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("closing listener: %v", err)
	}
	// The chart repository is unreachable too: an uninstall by release name must not need it.
	mo := operatorHandler(t, apiServer, apiServer)

	err = mo.Undeploy()
	if got := errors.GetCode(err); got != ErrOperatorClusterAccessCode {
		t.Fatalf("Undeploy() = %v (code %q), want %q", err, got, ErrOperatorClusterAccessCode)
	}
}

// firstLine returns the first line of err's instance text.
func firstLine(err error) string {
	if e, ok := errors.AsErrorV2(err); ok {
		if lines := errors.NonEmptyLines(e.LongDescription); len(lines) > 0 {
			return lines[0]
		}
	}
	return err.Error()
}
