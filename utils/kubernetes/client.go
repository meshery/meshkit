package kubernetes

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/kubernetes/cloudauth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DetectKubeConfig detects the kubeconfig for the kubernetes cluster and returns it.
// It delegates to detectKubeConfig, discarding the clientcmd.ClientConfig loader;
// use detectKubeConfig directly when the loader must be retained (e.g. to keep
// exec-based credential plugins working across requests).
func DetectKubeConfig(configfile []byte) (config *rest.Config, err error) {
	config, _, err = detectKubeConfig(configfile)
	return config, err
}

// detectKubeConfig is the implementation behind DetectKubeConfig. In addition
// to the resolved *rest.Config, it also returns the clientcmd.ClientConfig
// loader that produced it, when one was used (i.e. for every source except
// an in-cluster config). Callers that only need the rest.Config should use
// DetectKubeConfig; callers that need to preserve authentication mechanisms
// which must be re-resolved per request - such as exec-based credential
// plugins - should retain the loader as well.
func detectKubeConfig(configfile []byte, authOpts ...cloudauth.Option) (config *rest.Config, loader clientcmd.ClientConfig, err error) {
	if len(configfile) > 0 {
		var cfgFile []byte

		_, cfgFile, err = ProcessConfig(configfile, "")
		if err != nil {
			return nil, nil, err
		}

		config, loader, err = loadClientConfigFromKubeconfig(cfgFile, authOpts...)
		if err != nil {
			return nil, nil, wrapKubeConfigError(err)
		}
		return config, loader, nil
	}

	// If deployed within the cluster
	if config, err = rest.InClusterConfig(); err == nil {
		return config, nil, nil
	}

	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		// KUBECONFIG expresses an explicit target. Fail closed instead of falling
		// back to the default file, which could target another cluster.
		var cfgFile []byte
		_, cfgFile, err = ProcessConfig(kubeconfig, "")
		if err != nil {
			return nil, nil, err
		}
		config, loader, err = loadClientConfigFromKubeconfig(cfgFile, authOpts...)
		if err != nil {
			return nil, nil, wrapKubeConfigError(err)
		}
		return config, loader, nil
	}

	// Look for kubeconfig at the default path
	path := filepath.Join(utils.GetHome(), ".kube", "config")
	var cfgFile []byte
	_, cfgFile, err = ProcessConfig(path, "")
	if err != nil {
		return nil, nil, err
	}
	if config, loader, err = loadClientConfigFromKubeconfig(cfgFile, authOpts...); err == nil {
		return config, loader, nil
	}

	return nil, nil, wrapKubeConfigError(err)
}

func wrapKubeConfigError(err error) error {
	var meshErr *meshkiterrors.Error
	var meshErrV2 *meshkiterrors.ErrorV2
	if stderrors.As(err, &meshErr) || stderrors.As(err, &meshErrV2) {
		return err
	}
	return ErrRestConfigFromKubeConfig(err)
}

// loadClientConfigFromKubeconfig parses the given kubeconfig bytes into a
// clientcmd.ClientConfig loader and resolves it into a *rest.Config.
// Recognized EKS/GKE auth configurations are handled in-process via cloudauth
// while unknown exec plugins stay untouched on the client-go loader (#986).
func loadClientConfigFromKubeconfig(kubeconfig []byte, authOpts ...cloudauth.Option) (*rest.Config, clientcmd.ClientConfig, error) {
	loader, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	rawCfg, err := loader.RawConfig()
	if err == nil {
		if authInfo := activeAuthInfo(rawCfg); authInfo != nil {
			tokenSource, _, matched, resolveErr := cloudauth.ResolveAuthInfo(context.Background(), authInfo, authOpts...)
			if resolveErr != nil {
				return nil, nil, resolveErr
			}
			if matched {
				cloudLoader := newCloudAuthClientConfig(loader, tokenSource)
				config, err := cloudLoader.ClientConfig()
				if err != nil {
					return nil, nil, err
				}
				return config, cloudLoader, nil
			}
		}
	}

	config, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	return config, loader, nil
}

func activeAuthInfo(rawCfg clientcmdapi.Config) *clientcmdapi.AuthInfo {
	ctxName := rawCfg.CurrentContext
	if ctxName != "" {
		if ctx, ok := rawCfg.Contexts[ctxName]; ok && ctx != nil && ctx.AuthInfo != "" {
			return rawCfg.AuthInfos[ctx.AuthInfo]
		}
	}
	if len(rawCfg.Contexts) == 1 {
		for _, ctx := range rawCfg.Contexts {
			if ctx != nil && ctx.AuthInfo != "" {
				return rawCfg.AuthInfos[ctx.AuthInfo]
			}
		}
	}
	return nil
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
