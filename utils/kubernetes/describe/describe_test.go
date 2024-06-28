package describe

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/describe"
)

// KubeClient interface for mock client
type KubeClient interface {
	GetRestConfig() *rest.Config
}

// MockClient implements a mock Kubernetes client
type MockClient struct {
	RestConfig rest.Config
}

func (c *MockClient) GetRestConfig() *rest.Config {
	return &c.RestConfig
}

// MockDescriberFor returns a mock describer
func MockDescriberFor(kind schema.GroupKind, c *rest.Config) (describe.ResourceDescriber, bool) {
	return mockDescriber{}, true
}

// mockDescriber is a mock implementation of describe.ResourceDescriber
type mockDescriber struct{}

func (mockDescriber) Describe(namespace, name string, describerSettings describe.DescriberSettings) (string, error) {
	if name == "test-invalid" {
		return "", ErrGetDescriberFunc()
	}
	return "mock description", nil
}

// DescribeWithMock allows injection of a custom DescriberFor function
func DescribeWithMock(client KubeClient, options DescriberOptions, describerFor func(schema.GroupKind, *rest.Config) (describe.ResourceDescriber, bool)) (string, error) {
	kind := ResourceMap[options.Type]
	describer, ok := describerFor(kind, client.GetRestConfig())
	if !ok {
		return "", ErrGetDescriberFunc()
	}
	describerSetting := describe.DescriberSettings{
		ShowEvents: options.ShowEvents,
		ChunkSize:  options.ChunkSize,
	}
	output, err := describer.Describe(options.Namespace, options.Name, describerSetting)
	if err != nil {
		return "", err
	}
	return output, nil
}

func TestDescribe(t *testing.T) {
	// Creating a mock client
	client := &MockClient{}
	tests := []struct {
		name    string
		options DescriberOptions
		wantErr bool
	}{
		{
			name: "Describe Pod",
			options: DescriberOptions{
				Name:       "test-pod",
				Namespace:  "default",
				ShowEvents: false,
				ChunkSize:  500,
				Type:       Pod,
			},
			wantErr: false,
		},
		{
			name: "Invalid Resource Type",
			options: DescriberOptions{
				Name:       "test-invalid",
				Namespace:  "default",
				ShowEvents: false,
				ChunkSize:  500,
				Type:       999, // Invalid type
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DescribeWithMock(client, tt.options, MockDescriberFor)
			if (err != nil) != tt.wantErr {
				t.Errorf("Describe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("Describe() = %v, want non-empty output", got)
			}
		})
	}
}
