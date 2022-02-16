package manifests

import (
	"github.com/meshery/meshkit/utils"
	k8s "github.com/meshery/meshkit/utils/kubernetes"
)

func GetFromManifest(url string, resource int, cfg Config) (*Component, error) {
	manifest, err := utils.ReadFileSource(url)
	if err != nil {
		return nil, err
	}
	comp, err := GenerateComponents(manifest, resource, cfg)
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
	comp, err := GenerateComponents(manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}
