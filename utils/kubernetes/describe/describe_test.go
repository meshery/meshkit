package describe

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	meshkitkube "github.com/layer5io/meshkit/utils/kubernetes"
)

type MockClient struct {
	Object       string
	Method       string
	RequestUrl   string
	ResponseCode int
	Response     string
}

func StartKindCluster() error {
	cmd := exec.Command("kind", "create", "cluster", "--config", "kind-cluster-config.yml")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start kind cluster: %v", err)
	}
	return nil
}

func DeleteKindCluster() error {
	cmd := exec.Command("kind", "delete", "cluster")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete Kind cluster: %v", err)
	}
	return nil
}
func TestDescribe(t *testing.T) {
	// Start Kind cluster to create a cluster
	err := StartKindCluster()
	if err != nil {
		t.Fatalf("Failed to start Kind cluster: %v", err)
	}
	log.Println("Kind cluster created")

	// Defer deleting the Kind cluster until the test is finished
	defer func() {
		err := DeleteKindCluster()
		if err != nil {
			log.Printf("Failed to delete Kind cluster: %v", err)
		}
		log.Println("Kind cluster deleted")
	}()

	//http time out to handle request and response
	timeout := 300 * time.Second
	clienttime := &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport,
	}
	httpmock.ActivateNonDefault(clienttime)
	//set up mock client  to handle api request and return expected responses
	mckpod := MockClient{
		Object:       "Pod",
		Method:       "GET",
		RequestUrl:   "api/v1/namespaces",
		ResponseCode: 200,
		Response:     "Mock response for pods",
	}
	mckDeployment := MockClient{
		Object:       "Deployment",
		Method:       "POST",
		RequestUrl:   "api/v1/namespaces",
		ResponseCode: 200,
		Response:     "Mock response for deployments",
	}
	mckJob := MockClient{
		Object:       "Job",
		Method:       "GET",
		RequestUrl:   "api/v1/namespaces",
		ResponseCode: 200,
		Response:     "Mock response for job",
	}
	mckcronjob := MockClient{
		Object:       "Cronjob",
		Method:       "PUT",
		RequestUrl:   "api/v1/namespaces",
		ResponseCode: 200,
		Response:     "Mock response for cronjobs",
	}
	//create a dummy kubeconfig to create a k8 client 
	kubeconfig := "test.yml"
	kubeconfigBytes, err := os.ReadFile(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to read kubeconfig file: %v", err)
	}
	log.Println("Kubeconfig loaded")
	meshclient, err := meshkitkube.New(kubeconfigBytes)
	if err != nil {
		log.Println("error in loading Kube configs for tests", err)
	}
	//create test cases
	testCases := []struct {
		Name          string
		Options       DescriberOptions
		ExpectedError error
		clientApi     MockClient
	}{
		{
			Name: "Pod",
			Options: DescriberOptions{
				Name:      "mypod",
				Namespace: "test",
				Type:      Pod,
			},
			ExpectedError: nil,
			clientApi:     mckpod,
		},
		{
			Name: "Deployment",
			Options: DescriberOptions{
				Name:      "deployment",
				Namespace: "test",
				Type:      Deployment,
			},
			ExpectedError: nil,
			clientApi:     mckDeployment,
		},
		{
			Name: "Job",
			Options: DescriberOptions{
				Name:      "job",
				Namespace: "test",
				Type:      Job,
			},
			ExpectedError: nil,
			clientApi:     mckJob,
		},
		{
			Name: "CronJob",
			Options: DescriberOptions{
				Name:      "cronjob",
				Namespace: "test",
				Type:      CronJob,
			},
			ExpectedError: nil,
			clientApi:     mckcronjob,
		},
	}
	// run test cases
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Register mock responders for each mock client
			httpmock.RegisterResponder(tc.clientApi.Method, tc.clientApi.RequestUrl,
				httpmock.NewStringResponder(tc.clientApi.ResponseCode, tc.clientApi.Response))
			// Set a custom timeout for the HTTP client used by httpmock
			output, err := Describe(meshclient, tc.Options)
			// append the response with the output
			output += " " + tc.clientApi.Response
			if err != nil {
				t.Errorf("Testcase failed for %v, couldn't get %v got %v, ", tc.Name, tc.ExpectedError, err)
			}
		})
	}
	httpmock.DeactivateAndReset()
}
