package kubernetes

import (
    "fmt"
    "testing"
)

func TestDetectKubeConfig(t *testing.T) {
    // Test case 1: Insecure TLS verification for context1
    testKubeConfig1 := []byte(`
apiVersion: v1
clusters:
- name: cluster1
  cluster:
    server: https://localhost:9443
    insecure-skip-tls-verify: true
contexts:
- context:
    cluster: cluster1
    user: user1
  name: context1
current-context: context1
kind: Config
preferences: {}
users:
- name: user1
  user:
`)
    config1, err1 := DetectKubeConfig(testKubeConfig1)
    if err1 != nil {
        t.Errorf("Test case 1: Error while detecting kubeconfig: %v", err1)
    }
    if config1 == nil {
        t.Errorf("Test case 1: Config should not be nil")
    }
    if !config1.TLSClientConfig.Insecure {
        t.Errorf("Test case 1: TLS verification should be skipped, but it's not")
    }

    // Test case 2: Secure TLS verification for context2
    testKubeConfig2 := []byte(`
apiVersion: v1
clusters:
- name: cluster2
  cluster:
    server: https://localhost:9443
    insecure-skip-tls-verify: false
contexts:
- context:
    cluster: cluster2
    user: user2
  name: context2
current-context: context2
kind: Config
preferences: {}
users:
- name: user2
  user:
`)
    config2, err2 := DetectKubeConfig(testKubeConfig2)
    if err2 != nil {
        t.Errorf("Test case 2: Error while detecting kubeconfig: %v", err2)
    }
    if config2 == nil {
        t.Errorf("Test case 2: Config should not be nil")
    }
    if config2.TLSClientConfig.Insecure {
        t.Errorf("Test case 2: TLS verification should not be skipped, but it is")
    }

    // Test case 3: Multi-context kubeconfig with mixed TLS settings
    testKubeConfig3 := []byte(`
apiVersion: v1
clusters:
- name: cluster1
  cluster:
    server: https://localhost:9443
    insecure-skip-tls-verify: true
- name: cluster2
  cluster:
    server: https://localhost:9443
    insecure-skip-tls-verify: false
contexts:
- context:
    cluster: cluster1
    user: user1
  name: context1
- context:
    cluster: cluster2
    user: user2
  name: context2
current-context: context1
kind: Config
preferences: {}
users:
- name: user1
  user:
- name: user2
  user:
`)
    config3, err3 := DetectKubeConfig(testKubeConfig3)
    if err3 != nil {
        t.Errorf("Test case 3: Error while detecting kubeconfig: %v", err3)
    }
    if config3 == nil {
        t.Errorf("Test case 3: Config should not be nil")
    }
    if !config3.TLSClientConfig.Insecure {
        t.Errorf("Test case 3: TLS verification should be skipped, but it's not")
    }

    event := CustomEvent{
        EventType: "Warning",
        Level:     "Warning",
        Message:   "Insecure connection to Kubernetes cluster detected",
    }

    handleCustomEvent(event)


    // Print whether TLS verification is skipped or not for each test case
    fmt.Printf("Test case 1: TLS verification is skipped (Insecure: %v)\n", config1.TLSClientConfig.Insecure)
    fmt.Printf("Test case 2: TLS verification is not skipped (Insecure: %v)\n", config2.TLSClientConfig.Insecure)
    fmt.Printf("Test case 3: TLS verification is skipped (Insecure: %v)\n", config3.TLSClientConfig.Insecure)

}