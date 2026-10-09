package expose

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGenerateService(t *testing.T) {
	got, err := generateService(serviceConfig{
		selectorsMap: map[string]string{"app": "meshery"},
		labelsMap:    map[string]string{"team": "meshery"},
		protocolsMap: map[string]string{"80": "TCP"},
		portsSlice:   []string{"80", "443"},
		Config: Config{
			Name:            "meshery",
			Namespace:       "default",
			Type:            LoadBalancer,
			LoadBalancerIP:  "10.0.0.10",
			SessionAffinity: ClientIP,
			ClusterIP:       "10.0.0.20",
		},
	})
	if err != nil {
		t.Fatalf("generateService() error = %v", err)
	}

	wantPorts := []v1.ServicePort{
		{Name: "port-1", Port: 80, TargetPort: intstrFromInt(80), Protocol: v1.ProtocolTCP},
		{Name: "port-2", Port: 443, TargetPort: intstrFromInt(443), Protocol: v1.ProtocolTCP},
	}
	if !reflect.DeepEqual(got.Spec.Ports, wantPorts) || got.Spec.Type != v1.ServiceTypeLoadBalancer || got.Spec.SessionAffinity != v1.ServiceAffinityClientIP {
		t.Errorf("generateService() produced unexpected service: %#v", got.Spec)
	}
	if got.Spec.Selector["app"] != "meshery" || got.Spec.LoadBalancerIP != "10.0.0.10" {
		t.Errorf("generateService() metadata/config mismatch: %#v", got)
	}
}

func TestGenerateServiceInvalidInput(t *testing.T) {
	if _, err := generateService(serviceConfig{portsSlice: []string{"not-a-port"}}); err == nil {
		t.Error("generateService() error = nil, want invalid port error")
	}
	if _, err := generateService(serviceConfig{portsSlice: []string{"80"}, Config: Config{SessionAffinity: "invalid"}}); err == nil {
		t.Error("generateService() error = nil, want invalid session affinity error")
	}
}

func TestCanBeExposed(t *testing.T) {
	if err := canBeExposed(schema.GroupKind{Group: "apps", Kind: "Deployment"}); err != nil {
		t.Errorf("canBeExposed() returned error for Deployment: %v", err)
	}
	if err := canBeExposed(schema.GroupKind{Group: "batch", Kind: "Job"}); err == nil {
		t.Error("canBeExposed() error = nil for unsupported Job")
	}
}

func TestResourcePortAndProtocolExtraction(t *testing.T) {
	pod := &v1.Pod{Spec: v1.PodSpec{Containers: []v1.Container{{Ports: []v1.ContainerPort{
		{ContainerPort: 8080},
		{ContainerPort: 9090, Protocol: v1.ProtocolUDP},
	}}}}}
	ports, err := portsForObject(pod)
	if err != nil || !reflect.DeepEqual(ports, []string{"8080", "9090"}) {
		t.Fatalf("portsForObject() = %v, %v", ports, err)
	}
	protocols, err := protocolsForObject(pod)
	if err != nil || !reflect.DeepEqual(protocols, map[string]string{"8080": "TCP", "9090": "UDP"}) {
		t.Fatalf("protocolsForObject() = %v, %v", protocols, err)
	}

	service := &v1.Service{Spec: v1.ServiceSpec{Ports: []v1.ServicePort{{Port: 80}, {Port: 53, Protocol: v1.ProtocolUDP}}}}
	servicePorts, err := portsForObject(service)
	if err != nil || !reflect.DeepEqual(servicePorts, []string{"80", "53"}) {
		t.Fatalf("portsForObject(service) = %v, %v", servicePorts, err)
	}
	serviceProtocols, err := protocolsForObject(service)
	if err != nil || !reflect.DeepEqual(serviceProtocols, map[string]string{"80": "TCP", "53": "UDP"}) {
		t.Fatalf("protocolsForObject(service) = %v, %v", serviceProtocols, err)
	}
}

func TestMapBasedSelectorForObject(t *testing.T) {
	deployment := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "meshery"}}}}
	got, err := mapBasedSelectorForObject(deployment)
	if err != nil || !reflect.DeepEqual(got, map[string]string{"app": "meshery"}) {
		t.Fatalf("mapBasedSelectorForObject() = %v, %v", got, err)
	}
	if _, err := mapBasedSelectorForObject(&v1.Node{}); err == nil {
		t.Error("mapBasedSelectorForObject(Node) error = nil, want unsupported-object error")
	}
}

// Kept local to make the expected ServicePort values readable in the test.
func intstrFromInt(value int) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.Int, IntVal: int32(value)}
}
