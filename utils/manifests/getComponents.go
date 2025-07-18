package manifests

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/meshery/meshkit/utils"
	k8s "github.com/meshery/meshkit/utils/kubernetes"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
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
	if strings.HasPrefix(url, "oci://") {
		return getCrdsFromOCI(url)
	}
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

func getCrdsFromOCI(ociURL string) ([]string, error) {
	settings := cli.New()
	tmpDir, err := os.MkdirTemp("", "oci-chart-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {}); err != nil {
		return nil, err
	}

	pull := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pull.DestDir = tmpDir
	pull.Settings = settings
	pull.Untar = true

	_, err = pull.Run(ociURL)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	chartDir := filepath.Join(tmpDir, entries[0].Name())

	chart, err := loader.Load(chartDir)
	if err != nil {
		return nil, err
	}

	var crds []string
	for _, f := range chart.CRDs() {
		crds = append(crds, string(f.Data))
	}
	return removeNonCrdValues(crds), nil
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
