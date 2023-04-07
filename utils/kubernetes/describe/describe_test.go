package describe

import (
	"testing"

	meshkitkube "github.com/layer5io/meshkit/utils/kubernetes"
)

/*
The MockDescriber is used in the implementation of the Describe function to create a mock output that is returned when the
function is called with specific options
it takes in a Client object and a DescriberOptions object, and returns a string and an error
*/
type MockDescriber struct {
	DescribeFunc func(*meshkitkube.Client, DescriberOptions) (string, error)
}

// Describe method of the MockDescriber simply calls DescribeFunc and returns the result.
func (m *MockDescriber) Describe(client *meshkitkube.Client, options DescriberOptions) (string, error) {
	return m.DescribeFunc(client, options)
}

func TestDescribe(t *testing.T) {
	//meshkitkube.Client  provides the ability to interact with the Kubernetes API server

	//set up mock client client to return expected responses
	mockClient := meshkitkube.Client{}

	//create test cases
	testCases := []struct {
		Name           string
		Options        DescriberOptions
		DescribeFunc   func(*meshkitkube.Client, DescriberOptions) (string, error)
		ExpectedOutput string
		ExpectedError  error
	}{
		{
			Name: "describe pod",
			Options: DescriberOptions{
				Name:      "test-pod",
				Namespace: "test-namespace",
				Type:      Pod,
			},
			/*
				DescribeFunc field is a function that takes in a Client
				object and a DescriberOptions object, and returns a string and an error
			*/
			DescribeFunc: func(client *meshkitkube.Client, options DescriberOptions) (string, error) {
				return "Name: test-pod\nNamespace: test-namespace\n", nil
			},
			ExpectedOutput: "Name: test-pod\nNamespace: test-namespace\n",
			ExpectedError:  nil,
		},
		{
			Name: "describe deployment",
			Options: DescriberOptions{
				Name:      "test-deployment",
				Namespace: "test-namespace",
				Type:      Deployment,
			},
			DescribeFunc: func(client *meshkitkube.Client, options DescriberOptions) (string, error) {
				return "Name: test-deployment\nNamespace: test-namespace\n", nil
			},
			ExpectedOutput: "Name: test-deployment\nNamespace: test-namespace\n",
			ExpectedError:  nil,
		},
	}

	//run test cases
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			//create a mockDescriber
			mockDescriber := &MockDescriber{
				DescribeFunc: tc.DescribeFunc,
			}
			output, err := mockDescriber.Describe(&mockClient, tc.Options)

			//check if the output and error match the expected values8
			if output != tc.ExpectedOutput {
				t.Errorf("Test case %s failed. Expected: %s, but got: %s", tc.Name, tc.ExpectedOutput, output)
			}
			if err != tc.ExpectedError {
				t.Errorf("Test case %s failed. Expected error: %v, but got: %v", tc.Name, tc.ExpectedError, err)
			}
		})
	}

}
