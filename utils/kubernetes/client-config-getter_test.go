package kubernetes

import (
	"os"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func testExecKubeConfig() []byte {
	return []byte(`apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://cluster.example.com
    insecure-skip-tls-verify: true
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
users:
- name: test-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: credential-plugin
      interactiveMode: Never
`)
}

func TestNewRetainsKubeConfigLoader(t *testing.T) {
	client, err := New(testExecKubeConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	loader := client.rawKubeConfigLoader()
	if loader == nil {
		t.Fatal("New() did not retain the kubeconfig loader")
	}

	rawConfig, err := loader.RawConfig()
	if err != nil {
		t.Fatalf("RawConfig() error = %v", err)
	}
	authInfo := rawConfig.AuthInfos["test-user"]
	if authInfo == nil || authInfo.Exec == nil {
		t.Fatal("retained kubeconfig loader lost exec authentication")
	}
	if authInfo.Exec.Command != "credential-plugin" {
		t.Fatalf("exec command = %q, want %q", authInfo.Exec.Command, "credential-plugin")
	}
}

func TestClientConfigRESTClientGetterPreservesLoader(t *testing.T) {
	_, loader, err := detectKubeConfig(testExecKubeConfig())
	if err != nil {
		t.Fatalf("detectKubeConfig() error = %v", err)
	}

	getter := newClientConfigRESTClientGetter(loader)
	if getter.ToRawKubeConfigLoader() != loader {
		t.Fatal("RESTClientGetter did not return the retained kubeconfig loader")
	}

	config, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider == nil || config.ExecProvider.Command != "credential-plugin" {
		t.Fatal("RESTClientGetter did not preserve exec authentication")
	}
}

func TestHelmRESTClientGetterPrefersRetainedLoader(t *testing.T) {
	config, loader, err := detectKubeConfig(testExecKubeConfig())
	if err != nil {
		t.Fatalf("detectKubeConfig() error = %v", err)
	}
	client := &Client{
		RestConfig:       *config,
		kubeConfigLoader: loader,
	}

	getter, cleanup, err := client.helmRESTClientGetter()
	if err != nil {
		t.Fatalf("helmRESTClientGetter() error = %v", err)
	}
	defer cleanup()

	if getter.ToRawKubeConfigLoader() != loader {
		t.Fatal("Helm did not receive the retained kubeconfig loader")
	}
	if _, ok := getter.(*genericclioptions.ConfigFlags); ok {
		t.Fatal("Helm reconstructed ConfigFlags despite a retained kubeconfig loader")
	}
}

func TestHelmRESTClientGetterFallsBackForDirectClient(t *testing.T) {
	client := &Client{}

	getter, cleanup, err := client.helmRESTClientGetter()
	if err != nil {
		t.Fatalf("helmRESTClientGetter() error = %v", err)
	}
	defer cleanup()

	configFlags, ok := getter.(*genericclioptions.ConfigFlags)
	if !ok {
		t.Fatalf("fallback getter type = %T, want *genericclioptions.ConfigFlags", getter)
	}
	if configFlags.KubeConfig == nil || *configFlags.KubeConfig != os.DevNull {
		t.Fatalf("fallback kubeconfig = %v, want %q", configFlags.KubeConfig, os.DevNull)
	}
}
