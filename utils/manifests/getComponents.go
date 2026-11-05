package manifests

import (
	"context"
	"encoding/json"
	"github.com/meshery/meshkit/utils"
	k8s "github.com/meshery/meshkit/utils/kubernetes"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
)

func GetFromManifest(ctx context.Context, url string, resource int, cfg Config) (*Component, error) {
	manifest, err := utils.ReadFileSource(url)
	if err != nil {
		return nil, err
	}
	comp, err := GenerateComponents(ctx, manifest, resource, cfg)
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func GetFromHelm(ctx context.Context, url string, resource int, cfg Config) (*Component, error) {
	manifest, err := k8s.GetManifestsFromHelm(url)
	if err != nil {
		return nil, err
	}
	comp, err := GenerateComponents(ctx, manifest, resource, cfg)
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

	manifest = repairYaml(manifest)

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

func repairYaml(doc string) string {
	fixed := strings.ReplaceAll(doc, "\napiVersion:", "\n---\napiVersion:")
	for strings.Contains(fixed, "\n---\n---\n") {
		fixed = strings.ReplaceAll(fixed, "\n---\n---\n", "\n---\n")
	}

	return fixed
}

func removeNonCrdValues(crds []string) []string {
	out := make([]string, 0)
	for _, crd := range crds {
		var crdMap map[string]interface{}
		if err := json.Unmarshal([]byte(crd), &crdMap); err != nil {
			continue
		}

		if crdMap["kind"] == "CustomResourceDefinition" {
			out = append(out, crd)
		}
	}
	return out
}
