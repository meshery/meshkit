package manifests

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/layer5io/meshkit/utils"
	k8s "github.com/layer5io/meshkit/utils/kubernetes"
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

func GetCrdsFromHelm(url string, pkgName string) ([]string, error) {
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
			errStr := err.Error()
			// Remove the first line of the error message which contains "yaml unmarshall error:"
			errStr = strings.TrimPrefix(errStr, strings.SplitN(errStr, "\n", 2)[0]+"\n")
			return nil, ErrYamlUnmarshalSyntax(errStr, pkgName)
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
