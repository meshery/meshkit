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
		cfgFile, err := processConfigFromFileLocation(kubeconfig)
		if err != nil {
			return nil, err
		}
		if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
			return config, err
		}
	}

	// Look for kubeconfig at the default path
	path := filepath.Join(utils.GetHome(), ".kube", "config")
	cfgFile, err := processConfigFromFileLocation(path)
	if err != nil {
		return nil, err
	}
	if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
		return config, err
	}

	return
}

// processConfigInternal performs the common processing steps on the kubeconfig.
func processConfigInternal(cfg *clientcmdapi.Config) ([]byte, error) {
	// Minify the kubeconfig
	if err := clientcmdapi.MinifyConfig(cfg); err != nil {
		return nil, ErrValidateConfig(err)
	}

	// Flatten the kubeconfig
	if err := clientcmdapi.FlattenConfig(cfg); err != nil {
		return nil, ErrValidateConfig(err)
	}

	// Validate the kubeconfig
	if err := clientcmd.Validate(*cfg); err != nil {
		return nil, ErrValidateConfig(err)
	}

	// Write the processed kubeconfig to a byte slice
	return clientcmd.Write(*cfg)
}

// processConfigFromFileLocation processes the kubeconfig from a file location.
func processConfigFromFileLocation(filename string) ([]byte, error) {
	cfg, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return nil, ErrLoadConfig(err)
	}
	return processConfigInternal(cfg)
}

// processConfig processes the kubeconfig provided as a byte slice.
func processConfig(configFile []byte) ([]byte, error) {
	cfg, err := clientcmd.Load(configFile)
	if err != nil {
		return nil, ErrLoadConfig(err)
	}
	return processConfigInternal(cfg)
}
