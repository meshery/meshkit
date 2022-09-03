package manifests

import (
	"context"
	"strings"

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

func GetCrdsFromHelm(url string) ([]string, error) {
	manifest, err := k8s.GetManifestsFromHelm(url)
	if err != nil {
		return nil, err
	}
	mans := strings.Split(manifest, "---")
	return removeNonCrdValues(mans), nil
}

func removeNonCrdValues(crds []string) []string {
	out := make([]string, 0)
	for _, crd := range crds {
		if crd != "" && crd != " " {
			out = append(out, crd)
		}
	}
	return out
}
