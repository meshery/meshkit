package manifests

import (
	"github.com/layer5io/meshkit/utils"
	k8 "github.com/layer5io/meshkit/utils/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetFromManifest(url string, resource int, cfg Config) (*Component, error) {
	manifest, err := utils.ReadFileSource(url)
	if err != nil {
		return nil, err
	}
	comp, err := generateComponents(manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func GetFromHelm(url string, resource int, cfg Config) (*Component, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, configOverrides)
	kcli, err := k8.New([]byte(kubeConfig.ConfigAccess().GetExplicitFile()))
	if err != nil {
		return nil, err
	}
	manifest, err := k8.GetManifestsFromHelm(kcli, url)
	if err != nil {
		return nil, err
	}
	comp, err := generateComponents(manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}
