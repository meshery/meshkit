package kubernetes

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils/kubernetes/cloudauth"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// testEKSServer is a well-formed EKS API endpoint. Tests using it never
	// send a request.
	testEKSServer = "https://0123456789ABCDEF0123456789ABCDEF.gr7.us-west-2.eks.amazonaws.com"
)

// testCAData returns a valid PEM certificate to stand in for a cluster CA.
func testCAData(t *testing.T) []byte {
	t.Helper()
	server := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	return serverCAData(server)
}

func serverCAData(server *httptest.Server) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
}

// execKubeConfig returns a kubeconfig whose user authenticates with exec.
// The cluster trusts caData, or skips TLS verification when caData is nil.
func execKubeConfig(t *testing.T, serverURL string, caData []byte, exec *clientcmdapi.ExecConfig) []byte {
	t.Helper()

	cluster := &clientcmdapi.Cluster{Server: serverURL, CertificateAuthorityData: caData}
	if caData == nil {
		cluster.InsecureSkipTLSVerify = true
	}
	config := clientcmdapi.NewConfig()
	config.Clusters["test-cluster"] = cluster
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{Exec: exec}
	config.Contexts["test-context"] = &clientcmdapi.Context{Cluster: "test-cluster", AuthInfo: "test-user"}
	config.CurrentContext = "test-context"

	data, err := clientcmd.Write(*config)
	if err != nil {
		t.Fatalf("clientcmd.Write() error = %v", err)
	}
	return data
}

func eksExec(command string) *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1beta1",
		Command:         command,
		Args:            []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "demo"},
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
	}
}

func gkeExec() *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1beta1",
		Command:         "gke-gcloud-auth-plugin",
		Args:            []string{"--use_application_default_credentials"},
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
	}
}

func TestRESTClientGetterLeavesUnknownExecPluginsToClientGo(t *testing.T) {
	for _, command := range []string{"kubelogin", "/usr/local/bin/custom"} {
		t.Run(command, func(t *testing.T) {
			kubeconfig := execKubeConfig(t, "https://cluster.example.com", nil, &clientcmdapi.ExecConfig{
				APIVersion:      "client.authentication.k8s.io/v1",
				Command:         command,
				InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
			})
			_, loader, err := detectKubeConfig(kubeconfig)
			if err != nil {
				t.Fatalf("detectKubeConfig() error = %v", err)
			}

			getter := newClientConfigRESTClientGetter(loader)
			config, err := getter.ToRESTConfig()
			if err != nil {
				t.Fatalf("ToRESTConfig() error = %v", err)
			}
			if config.ExecProvider == nil || config.ExecProvider.Command != command {
				t.Fatalf("ExecProvider = %#v, want the %q stanza left for client-go", config.ExecProvider, command)
			}
			if config.WrapTransport != nil {
				t.Fatal("ToRESTConfig() wrapped the transport of an unrecognized exec plugin")
			}

			config, err = DetectKubeConfig(kubeconfig)
			if err != nil {
				t.Fatalf("DetectKubeConfig() error = %v", err)
			}
			if config.ExecProvider == nil || config.ExecProvider.Command != command {
				t.Fatalf("DetectKubeConfig() ExecProvider = %#v, want the %q stanza intact", config.ExecProvider, command)
			}
		})
	}
}

func TestRESTClientGetterMintsTokensInProcess(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer in-process-token" {
			http.Error(w, "unexpected Authorization "+got, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gitVersion":"v1.34.0"}`))
	}))
	defer server.Close()

	// A GKE IP endpoint with its cluster CA, so the request goes end to end.
	_, loader, err := detectKubeConfig(execKubeConfig(t, server.URL, serverCAData(server), gkeExec()))
	if err != nil {
		t.Fatalf("detectKubeConfig() error = %v", err)
	}

	var factoryCalls atomic.Int32
	var gotSpec cloudauth.Spec
	auth := cloudauth.NewAuthenticator(cloudauth.WithTokenSourceFactory(
		func(_ context.Context, spec cloudauth.Spec) (oauth2.TokenSource, error) {
			factoryCalls.Add(1)
			gotSpec = spec
			return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "in-process-token"}), nil
		}))
	getter := newAuthenticatingRESTClientGetter(loader, auth)

	config, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("ToRESTConfig() kept ExecProvider; client-go would fork gke-gcloud-auth-plugin")
	}
	if config.BearerToken != "" {
		t.Fatalf("ToRESTConfig() set a static BearerToken %q", config.BearerToken)
	}
	if config.QPS != 50 || config.Burst != 100 {
		t.Fatalf("REST config rate limits = (%v, %d), want (50, 100)", config.QPS, config.Burst)
	}
	if gotSpec.Provider != cloudauth.ProviderGKE {
		t.Fatalf("factory spec = %#v, want GKE", gotSpec)
	}

	discoveryClient, err := getter.ToDiscoveryClient()
	if err != nil {
		t.Fatalf("ToDiscoveryClient() error = %v", err)
	}
	if _, err := discoveryClient.ServerVersion(); err != nil {
		t.Fatalf("ServerVersion() error = %v", err)
	}
	if requests.Load() == 0 {
		t.Fatal("test server saw no request")
	}

	// Helm resolves its own rest.Config through ToRawKubeConfigLoader; it must
	// get the same in-process authentication, while the raw kubeconfig keeps
	// the exec stanza unchanged.
	helmConfig, err := getter.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		t.Fatalf("ToRawKubeConfigLoader().ClientConfig() error = %v", err)
	}
	if helmConfig.ExecProvider != nil || helmConfig.WrapTransport == nil {
		t.Fatal("ToRawKubeConfigLoader().ClientConfig() did not apply in-process authentication")
	}
	rawConfig, err := getter.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		t.Fatalf("RawConfig() error = %v", err)
	}
	if user := rawConfig.AuthInfos["test-user"]; user == nil || user.Exec == nil || user.Exec.Command != "gke-gcloud-auth-plugin" {
		t.Fatal("RawConfig() lost the exec stanza")
	}

	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("token source factory calls = %d, want 1 (the getter must share one source per cluster)", got)
	}
}

func TestRESTClientGetterBindsEKSStanzaToEndpoint(t *testing.T) {
	exec := eksExec("aws")
	exec.Args = []string{"eks", "get-token", "--cluster-name", "demo"}
	_, loader, err := detectKubeConfig(execKubeConfig(t, testEKSServer, testCAData(t), exec))
	if err != nil {
		t.Fatalf("detectKubeConfig() error = %v", err)
	}

	var gotSpec cloudauth.Spec
	auth := cloudauth.NewAuthenticator(cloudauth.WithTokenSourceFactory(
		func(_ context.Context, spec cloudauth.Spec) (oauth2.TokenSource, error) {
			gotSpec = spec
			return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "in-process-token"}), nil
		}))
	config, err := newAuthenticatingRESTClientGetter(loader, auth).ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("ToRESTConfig() kept the EKS exec stanza for an EKS endpoint")
	}
	if gotSpec.Provider != cloudauth.ProviderEKS || gotSpec.EKS.ClusterID != "demo" || gotSpec.EKS.Region != "us-west-2" {
		t.Fatalf("factory spec = %#v, want EKS cluster demo in us-west-2 (region from the endpoint)", gotSpec)
	}
}

func TestNewMintsEKSTokensForDirectAndHelmPaths(t *testing.T) {
	// New uses the package default authenticator, which with no AWS
	// credentials and no aws binary would fail; give the process static
	// credentials so it mints in process without the network.
	isolateAWSEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	exec := eksExec("/nonexistent/bin/aws")
	exec.Args = []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "direct-and-helm"}

	client, err := New(execKubeConfig(t, testEKSServer, testCAData(t), exec))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.RestConfig.ExecProvider != nil || client.RestConfig.WrapTransport == nil {
		t.Fatal("New() did not replace the EKS exec stanza with in-process minting")
	}
	config, err := client.getRESTClientGetter().ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider != nil || config.WrapTransport == nil {
		t.Fatal("the Helm RESTClientGetter did not apply in-process minting")
	}
}

func TestDetectKubeConfigCloudPluginWithoutCredentials(t *testing.T) {
	isolateAWSEnvironment(t)
	// An absolute path that does not exist, so the aws CLI cannot be found
	// even on a workstation that has it installed.
	kubeconfig := execKubeConfig(t, testEKSServer, testCAData(t), eksExec("/nonexistent/bin/aws"))

	_, err := DetectKubeConfig(kubeconfig)
	if err == nil {
		t.Fatal("DetectKubeConfig() succeeded without cloud credentials or plugin")
	}
	if got := errors.GetCode(err); got != cloudauth.ErrInProcessCloudAuthUnavailableCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, cloudauth.ErrInProcessCloudAuthUnavailableCode, err)
	}

	if _, err := New(kubeconfig); errors.GetCode(err) != cloudauth.ErrInProcessCloudAuthUnavailableCode {
		t.Fatalf("New() error = %v, want code %q", err, cloudauth.ErrInProcessCloudAuthUnavailableCode)
	}
}

func TestEveryPathRefusesUntrustedEndpoints(t *testing.T) {
	// The process has AWS credentials; only the endpoint rules keep them
	// from the kubeconfig's server.
	isolateAWSEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	for name, kubeconfig := range map[string][]byte{
		"attacker host":               execKubeConfig(t, "https://attacker.example.com", testCAData(t), eksExec("/nonexistent/bin/aws")),
		"TLS verification disabled":   execKubeConfig(t, testEKSServer, nil, eksExec("/nonexistent/bin/aws")),
		"GKE stanza on attacker host": execKubeConfig(t, "https://attacker.example.com", testCAData(t), gkeExec()),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			if _, err := DetectKubeConfig(kubeconfig); errors.GetCode(err) != cloudauth.ErrUntrustedClusterEndpointCode {
				t.Fatalf("DetectKubeConfig() error = %v, want code %q", err, cloudauth.ErrUntrustedClusterEndpointCode)
			}
			if _, err := New(kubeconfig); errors.GetCode(err) != cloudauth.ErrUntrustedClusterEndpointCode {
				t.Fatalf("New() error = %v, want code %q", err, cloudauth.ErrUntrustedClusterEndpointCode)
			}
			_, loader, err := detectKubeConfig(kubeconfig)
			if err != nil {
				t.Fatalf("detectKubeConfig() error = %v", err)
			}
			helmLoader := newClientConfigRESTClientGetter(loader).ToRawKubeConfigLoader()
			if _, err := helmLoader.ClientConfig(); errors.GetCode(err) != cloudauth.ErrUntrustedClusterEndpointCode {
				t.Fatalf("Helm loader ClientConfig() error = %v, want code %q", err, cloudauth.ErrUntrustedClusterEndpointCode)
			}
		})
	}
}

func TestDetectKubeConfigDisabledInProcessCloudAuth(t *testing.T) {
	isolateAWSEnvironment(t)
	t.Setenv(cloudauth.DisableEnvVar, "true")
	kubeconfig := execKubeConfig(t, testEKSServer, testCAData(t), eksExec("/nonexistent/bin/aws"))

	config, err := DetectKubeConfig(kubeconfig)
	if err != nil {
		t.Fatalf("DetectKubeConfig() error = %v", err)
	}
	if config.ExecProvider == nil || config.ExecProvider.Command != "/nonexistent/bin/aws" {
		t.Fatalf("ExecProvider = %#v, want the exec stanza untouched when disabled", config.ExecProvider)
	}
	if config.WrapTransport != nil {
		t.Fatal("DetectKubeConfig() wrapped the transport with in-process minting disabled")
	}
}

// isolateAWSEnvironment points the AWS default credential chain at nothing,
// so tests never read the developer's AWS configuration or reach instance
// metadata.
func isolateAWSEnvironment(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", fmt.Sprintf("%s/config", dir))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", fmt.Sprintf("%s/credentials", dir))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, name := range []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		cloudauth.DisableEnvVar,
	} {
		t.Setenv(name, "")
	}
}
