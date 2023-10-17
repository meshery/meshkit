package kubernetes

import (
	"os"
	"path/filepath"

	"github.com/layer5io/meshkit/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DetectKubeConfig detects the kubeconfig for the kubernetes cluster and returns it
func DetectKubeConfig(configfile []byte) (config *rest.Config, err error) {
	if len(configfile) > 0 {
		var cfgFile []byte
		cfgFile, err = processConfig(configfile)
		if err != nil {
			return nil, err
		}

		if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
			return config, err
		}
	}

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
	path := filepath.Join(utils.GetHome(), ".kube", "config")
	if config, err = clientcmd.BuildConfigFromFlags("", path); err == nil {
		return config, err
	}

	return
}

func processConfig(configFile []byte) ([]byte, error) {
	cfg, err := clientcmd.Load(configFile)
	if err != nil {
		return nil, ErrLoadConfig(err)
	}

	err = clientcmdapi.MinifyConfig(cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	err = clientcmdapi.FlattenConfig(cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	err = clientcmd.Validate(*cfg)
	if err != nil {
		return nil, ErrValidateConfig(err)
	}

	return clientcmd.Write(*cfg)
}
