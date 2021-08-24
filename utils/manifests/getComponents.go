package manifests

import (
	"github.com/layer5io/meshkit/utils"
	k8s "github.com/layer5io/meshkit/utils/kubernetes"
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
	manifest, err := k8s.GetManifestsFromHelm(url)
	if err != nil {
		return nil, err
	}
	comp, err := generateComponents(manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}
