package kubernetes

import (
	"os"
	"path"

	"github.com/layer5io/meshkit/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DetectKubeConfig detects the kubeconfig for the kubernetes cluster and returns it
func DetectKubeConfig() (config *rest.Config, err error) {
	// If deployed within the cluster
	if config, err = rest.InClusterConfig(); err == nil {
		return config, err
	}

	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		if config, err = clientcmd.BuildConfigFromFlags("", kubeconfig); err == nil {
			return config, err
		}
	}

	// Look for kubeconfig at the default path
	path := path.Join(utils.GetHome(), ".kube", "config")
	if config, err = clientcmd.BuildConfigFromFlags("", path); err == nil {
		return config, err
	}

	return
}
