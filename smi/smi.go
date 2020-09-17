package smi

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/layer5io/learn-layer5/smi-conformance/conformance"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/layer5io/gokit/utils"
)

var (
	name      = "smi-conformance"
	helmPath  = "https://github.com/layer5io/learn-layer5/raw/75a6b26cf5a5defaa0629e9bd55780fa52a82d2b/charts/smi-conformance-0.1.2.tgz"
	namespace = "meshery"
)

type SmiTest struct {
	id             string
	adaptorVersion string
	adaptorName    string
	ctx            context.Context
	kubeClient     *kubernetes.Clientset
	kubeConfigPath string
	smiAddress     string
	annotations    map[string]string
	labels         map[string]string
}

type Response struct {
	Id                string    `json:"id,omitempty"`
	Date              string    `json:"date,omitempty"`
	MeshName          string    `json:"mesh_name,omitempty"`
	MeshVersion       string    `json:"mesh_version,omitempty"`
	CasesPassed       string    `json:"cases_passed,omitempty"`
	PassingPercentage string    `json:"passing_percentage,omitempty"`
	Status            string    `json:"status,omitempty"`
	MoreDetails       []*Detail `json:"more_details,omitempty"`
}

type Detail struct {
	SmiSpecification string `json:"smi_specification,omitempty"`
	SmiVersion       string `json:"smi_version,omitempty"`
	Time             string `json:"time,omitempty"`
	Assertions       string `json:"assertions,omitemtpy"`
	Result           string `json:"result,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Capability       string `json:"capability,omitempty"`
	Status           string `json:"status,omitempty"`
}

func New(ctx context.Context, id string, version string, name string, client *kubernetes.Clientset) (*SmiTest, error) {

	if len(name) < 2 {
		return nil, ErrSmiInit("Adaptor name is nil")
	}

	if client == nil {
		return nil, ErrSmiInit("Client set is nil")
	}

	test := &SmiTest{
		ctx:            ctx,
		id:             id,
		adaptorVersion: version,
		kubeClient:     client,
		kubeConfigPath: fmt.Sprintf("%s/.kube/config", utils.GetHome()),
		adaptorName:    name,
		labels:         make(map[string]string, 0),
		annotations:    make(map[string]string, 0),
	}

	return test, nil
}

func (test *SmiTest) Run(labels, annotations map[string]string) (Response, error) {

	if labels != nil {
		test.labels = labels
	}

	if annotations != nil {
		test.annotations = annotations
	}

	response := Response{
		Id:                test.id,
		Date:              time.Now().String(),
		MeshName:          test.adaptorName,
		MeshVersion:       test.adaptorVersion,
		CasesPassed:       "0",
		PassingPercentage: "0",
		Status:            "deploying",
	}

	err := test.installConformanceTool()
	if err != nil {
		response.Status = "installing"
		return response, ErrInstallSmi(err)
	}

	err = test.connectConformanceTool()
	if err != nil {
		response.Status = "connecting"
		return response, ErrConnectSmi(err)
	}

	err = test.runConformanceTest(&response)
	if err != nil {
		response.Status = "running"
		return response, ErrRunSmi(err)
	}

	err = test.deleteConformanceTool()
	if err != nil {
		response.Status = "deleting"
		return response, ErrDeleteSmi(err)
	}

	response.Status = "completed"
	return response, nil
}

// installConformanceTool installs the smi conformance tool
func (test *SmiTest) installConformanceTool() error {

	_, err := test.kubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      name,
				"meta.helm.sh/release-namespace": namespace,
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
			},
		}}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	localpath := "/tmp/smi-conformance.tar.gz"
	err = utils.DownloadFile(localpath, helmPath)
	if err != nil {
		return err
	}

	chart, err := loader.Load(localpath)
	if err != nil {
		return err
	}

	actionConfig := &action.Configuration{}
	if err := actionConfig.Init(kube.GetConfig(test.kubeConfigPath, "", namespace), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		return err
	}

	iCli := action.NewInstall(actionConfig)
	iCli.Namespace = namespace
	iCli.ReleaseName = name
	_, err = iCli.Run(chart, nil)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second) // Required for all the resources to be created

	return nil
}

// deleteConformanceTool deletes the smi conformance tool
func (test *SmiTest) deleteConformanceTool() error {
	err := test.kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// connectConformanceTool initiates the connection
func (test *SmiTest) connectConformanceTool() error {
	var host string
	var port int32

	svc, err := test.kubeClient.CoreV1().Services(namespace).Get(test.ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	nodes, err := test.kubeClient.CoreV1().Nodes().List(test.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	addresses := make(map[string]string, 0)
	for _, addr := range nodes.Items[0].Status.Addresses {
		addresses[string(addr.Type)] = addr.Address
	}
	host = addresses["ExternalIP"]
	port = svc.Spec.Ports[0].NodePort
	if tcpCheck(addresses["InternalIP"], port) {
		host = addresses["InternalIP"]
	}

	test.smiAddress = fmt.Sprintf("%s:%d", host, port)
	return nil
}

func tcpCheck(ip string, port int32) bool {
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		return true
	}
	return false
}

// runConformanceTest runs the conformance test
func (test *SmiTest) runConformanceTest(response *Response) error {

	cClient, err := conformance.CreateClient(context.TODO(), test.smiAddress)
	if err != nil {
		return err
	}
	defer cClient.Close()

	result, err := cClient.CClient.RunTest(context.TODO(), &conformance.Request{
		Annotations: test.annotations,
		Labels:      test.labels,
		Meshname:    test.adaptorName,
		Meshversion: test.adaptorVersion,
	})
	if err != nil {
		return err
	}

	response.CasesPassed = result.Casespassed
	response.PassingPercentage = result.Passpercent

	details := make([]*Detail, 0)

	for _, d := range result.Details {
		details = append(details, &Detail{
			SmiSpecification: d.Smispec,
			Time:             d.Time,
			Assertions:       d.Assertions,
			Result:           d.Result,
			Reason:           d.Reason,
			Capability:       d.Capability,
			Status:           d.Status,
		})
	}

	response.MoreDetails = details

	return nil
}
