package kubernetes

import (
	"fmt"
	"testing"
)

func TestDetectKubeConfig(t *testing.T) {
	t.Parallel()
	// Test case 1: Valid kubeconfig file with tls-skip-verify set to true
	configfile := []byte(`
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: REDACTED
    server: https://localhost:6443
    insecure-skip-tls-verify: false
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: REDACTED
    client-key-data: REDACTED
`)
	config, err := DetectKubeConfig(configfile)
	if err != nil {
		t.Errorf("DetectKubeConfig() failed with error: %v", err)
	}
	if config == nil {
		t.Errorf("DetectKubeConfig() returned nil config")
	}
	if config != nil && config.TLSClientConfig.CAData != nil {
		fmt.Print("Test case 1: CertificateAuthorityData should be empty, but it's not")
	}

	// Test case 2: Valid kubeconfig file with tls-skip-verify set to false
	configfile = []byte(`
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: REDACTED
    server: https://localhost:6443
    insecure-skip-tls-verify: false
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: REDACTED
    client-key-data: REDACTED
`)
	config, err = DetectKubeConfig(configfile)
	if err != nil {
		t.Errorf("DetectKubeConfig() failed with error: %v", err)
	}
	if config == nil {
		t.Errorf("DetectKubeConfig() returned nil config")
	}
	if config != nil && config.TLSClientConfig.CAData == nil {
		fmt.Print("Test case 2: CertificateAuthorityData should not be empty, but it is")
	}

	// Test case 3: Invalid kubeconfig file
	configfile = []byte(`invalid kubeconfig`)
	config, err = DetectKubeConfig(configfile)
	if err == nil {
		t.Errorf("DetectKubeConfig() did not return an error for invalid kubeconfig")
	}
	if config != nil {
		t.Errorf("DetectKubeConfig() returned non-nil config for invalid kubeconfig")
	}
	if config != nil && config.TLSClientConfig.CAData != nil {
		t.Errorf("Test case 3: CertificateAuthorityData should be empty, but it's not")
	}
}

func TestPublishEventWithChannelLeak(t *testing.T) {
	t.Parallel()
	// Simulate multiple subscribers
	numSubscribers := 10
	channels := make([]chan interface{}, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		channels[i] = make(chan interface{})
	}

	// Publish an event without closing the channels
	event := "test event with channel leak"
	publishEvent(event)

}
