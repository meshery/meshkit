package manifests

import (
	"encoding/json"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

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

func GetCrdsFromHelm(url string) ([]string, error) {
	manifest, err := k8s.GetManifestsFromHelm(url)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	var mans []string
	for {
		var parsedYaml map[string]interface{}
		if err := dec.Decode(&parsedYaml); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		b, err := json.Marshal(parsedYaml)
		if err != nil {
			return nil, err
		}
		mans = append(mans, string(b))
	}

	return removeNonCrdValues(mans), nil
}

func removeNonCrdValues(crds []string) []string {
	out := make([]string, 0)
	for _, crd := range crds {
		if crd != "" && crd != " " && crd != "null" {
			out = append(out, crd)
		}
	}
	return out
}
