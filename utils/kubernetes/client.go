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

		_, cfgFile, err = ProcessConfig(configfile, "")
		if err != nil {
			return nil, err
		}

		if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
			return config, nil
		}
	}

	// If deployed within the cluster
	if config, err = rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		_, cfgFile, err := ProcessConfig(kubeconfig, "")
		if err != nil {
			return nil, err
		}
		if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
			return config, nil
		}
	}

	// Look for kubeconfig at the default path
	path := filepath.Join(utils.GetHome(), ".kube", "config")
	_, cfgFile, err := ProcessConfig(path, "")
	if err != nil {
		return nil, err
	}
	if config, err = clientcmd.RESTConfigFromKubeConfig(cfgFile); err == nil {
		return config, nil
	}

	return nil, ErrRestConfigFromKubeConfig(err)
}

// ProcessConfig handles loading, validating, and optionally saving or returning a kubeconfig
func ProcessConfig(kubeConfig interface{}, outputPath string) (*clientcmdapi.Config, []byte, error) {
	var config *clientcmdapi.Config
	var err error

	// Load the Kubeconfig
	switch v := kubeConfig.(type) {
	case string:
		config, err = clientcmd.LoadFromFile(v)
	case []byte:
		config, err = clientcmd.Load(v)
	default:
		return nil, nil, ErrLoadConfig(err)
	}
	if err != nil {
		return nil, nil, ErrLoadConfig(err)
	}

	// Validate and Process the Config
	if err := clientcmdapi.MinifyConfig(config); err != nil {
		return nil, nil, ErrValidateConfig(err)
	}

	if err := clientcmdapi.FlattenConfig(config); err != nil {
		return nil, nil, ErrValidateConfig(err)
	}

	if err := clientcmd.Validate(*config); err != nil {
		return nil, nil, ErrValidateConfig(err)
	}

	// Convert the config to []byte
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return nil, nil, utils.ErrConvertToByte(err)
	}

	//Save the Processed config to a file
	if outputPath != "" {
		if err := clientcmd.WriteToFile(*config, outputPath); err != nil {
			return nil, nil, utils.ErrWriteFile(err, outputPath)
		}
	}

	return config, configBytes, nil
}
