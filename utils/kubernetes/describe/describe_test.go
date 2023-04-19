package describe

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	
)

// A MockClient meant to interact with the Kubernetes Api
type MockClient struct {
	Response      []byte //stores the response body that should be returned when the MockClient makes a request.
	RequestUrl    string // stores the URL that the Mock client should expect when the HTTP request is performed
	RequestMethod string // stores the HTTP request method made by the mock client
	RequestBody   []byte // stores the request body that the fake client should expect to receive when the HTTP request is made
	DescribeError error  //handle errors for Describe()

}

// the main role of the Prepend Reactor is to return httpErrors for Describerfunc
// PrependReactor adds a reactor function that gets called before every mock request is sent.
// The reactor function sets the HTTP status code to 404 if the Describe() function is called.
func (m *MockClient) PrependReactor(method string, Name string, reactorFunc func(*http.Request) (*http.Response, error)) {
	originalDo := m.Do

	_ = func(req http.Request) (*http.Response, error) {
		//call the reactor funtion for every reqeust
		resp, err := reactorFunc(&req)
		if err != nil || resp != nil {
			// If the reactor function returns a response or error, return it immediately
			return resp, err
		}
		// Otherwise, call the original Do() method
		return originalDo(req)
	}
}

// Do executes request and returns a response
func (m *MockClient) Do(req http.Request) (*http.Response, error) {
	m.RequestUrl = req.URL.String()
	m.RequestMethod = req.Method
	m.RequestBody, _ = ioutil.ReadAll(req.Body)

	if m.DescribeError != nil && strings.Contains(m.RequestUrl, "describe") {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewReader(m.Response)),
		}, m.DescribeError
	}
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewReader(m.Response)),
	}, m.DescribeError

}

func (m *MockClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	//calls the Do() to execute request and handle response
	return m.Do(*req)

}

func (m *MockClient) Post(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	//calls the Do() to execute request and handle response
	return m.Do(*req)
}

func (m *MockClient) Put(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return nil, err
	}
	//calls the Do() to execute request and handle response
	return m.Do(*req)
}

func (m *MockClient) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	//calls the Do() to execute request and handle response
	return m.Do(*req)
}

func (m *MockClient) Patch(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPatch, url, nil)
	if err != nil {
		return nil, err
	}
	return m.Do(*req)

}

/*
The MockDescriber is used in the implementation of the Describe function to create a mock output that is returned when the
function is called with specific options
it takes in a Client object and a DescriberOptions object, and returns a string and an error
*/
type MockDescriber struct {
	DescribeFunc func(MockClient, DescriberOptions) (string, error)
}

// Describe method of the MockDescriber simply calls DescribeFunc and returns the result.
func (m *MockDescriber) Describe(client MockClient, options DescriberOptions) (string, error) {
	return m.DescribeFunc(client, options)
}

func TestDescribe(t *testing.T) {
	//meshkitkube.Client  provides the ability to interact with the Kubernetes API server

	//set up mock client client to return expected responses
	mockClient := MockClient{}

	//create test cases
	testCases := []struct {
		Name           string
		Options        DescriberOptions
		DescribeFunc   func(MockClient, DescriberOptions) (string, error)
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
			DescribeFunc: func(client MockClient, options DescriberOptions) (string, error) {
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
			DescribeFunc: func(client MockClient, options DescriberOptions) (string, error) {
				return "Name: test-deployment\nNamespace: test-namespace\n", nil
			},
			ExpectedOutput: "Name: test-deployment\nNamespace: test-namespace\n",
			ExpectedError:  nil,
		},
	}
	// run test cases
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// create a mockDescriber
			mockDescriber := &MockDescriber{
				DescribeFunc: tc.DescribeFunc,
			}

			// add a reactor to the mock client to return an error if the DescribeFunc is called
			mockClient.PrependReactor("GET", "test-pod", func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("error for testpods")
			})

			mockClient.PrependReactor("GET", "test-deployment", func(*http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("error for deployment")
			})

			// call the Describe method
			output, err := mockDescriber.Describe(mockClient, tc.Options)

			// check if the output and error match the expected values
			if output != tc.ExpectedOutput {
				t.Errorf("Test case %s failed. Expected: %s, but got: %s", tc.Name, tc.ExpectedOutput, output)
			}
			if err != tc.ExpectedError {
				t.Errorf("Test case %s failed. Expected error: %v, but got: %v", tc.Name, tc.ExpectedError, err)
			}
		})
	}

}
