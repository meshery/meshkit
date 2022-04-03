package kubernetes

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func (c *Client) GetKubeConfig() (*models.Kubeconfig, error) {
	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(utils.GetHome(), ".kube", "config")
	}

	var config *models.Kubeconfig
	file, err := os.ReadFile(kubeconfig)
	if err != nil {
		err = errors.Wrap(err, "could not read kubeconfig:")
		return nil, err
	}
	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Client) GetCurrentContext() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}

	return config.CurrentContext, nil
}

func WriteEKSConfig(clusterName, region, configPath string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	eksSvc := eks.New(sess)

	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}
	result, err := eksSvc.DescribeCluster(input)
	if err != nil {
		log.Fatalf("Error calling DescribeCluster: %v", err)
	}

	cname := *result.Cluster.Arn
	endpt := *result.Cluster.Endpoint
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   endpt,
		CertificateAuthorityData: []byte(*result.Cluster.CertificateAuthority.Data),
	}
	contexts := make(map[string]*clientcmdapi.Context)
	contexts[cname] = &clientcmdapi.Context{
		Cluster: cname,
	}

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: cname,
	}

	prevConfigbytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	tmpArch := "tmp/kube/config"
	err = os.WriteFile(tmpArch, prevConfigbytes, 0644)
	if err != nil {
		return err
	}
	log.Printf("Warning: Overwriting previous config, archived config at %s", tmpArch)
	if err := clientcmd.WriteToFile(clientConfig, configPath); err != nil {
		return err
	}
	// O_CREATE   O_TRUNC
	return nil
}
