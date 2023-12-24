package generic

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils/component"
	"github.com/layer5io/meshkit/utils/manifests"
	"helm.sh/helm/v3/pkg/chart/loader"
)

type GithubRepo struct {
	RepoURL  string `yaml:"url"`
	ChartDir string `yaml:"chart_dir"`
	Name     string `yaml:"name"`
}

func (repo *GithubRepo) getCRDs() ([]string, string, error) {
	id := strings.ReplaceAll(repo.RepoURL, "/", "-")
	path := filepath.Join(os.TempDir(), id, strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:      repo.RepoURL,
		Progress: os.Stdout,
	})
	chart, err := loader.Load(filepath.Join(path, repo.ChartDir))
	if err != nil {
		return nil, "", err
	}
	var manifest string = ""
	for _, crdobject := range chart.CRDObjects() {
		manifest += "\n---\n"
		manifest += string(crdobject.File.Data)
	}
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	var mans []string
	for {
		var parsedYaml map[string]interface{}
		if err := dec.Decode(&parsedYaml); err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", err
		}
		b, err := json.Marshal(parsedYaml)
		if err != nil {
			return nil, "", err
		}
		mans = append(mans, string(b))
	}

	return removeNonCrdValues(mans), chart.AppVersion(), nil
}

func (repo *GithubRepo) GenerateComponents() ([]v1alpha1.ComponentDefinition, error) {
	components := make([]v1alpha1.ComponentDefinition, 0)
	crds, version, err := repo.getCRDs()
	if err != nil {
		return components, ErrComponentGenerate(err)
	}
	for _, crd := range crds {
		comp, err := component.Generate(crd)
		if err != nil {
			continue
		}
		if comp.Metadata == nil {
			comp.Metadata = make(map[string]interface{})
		}
		if comp.Model.Metadata == nil {
			comp.Model.Metadata = make(map[string]interface{})
		}
		comp.Model.Metadata["source_uri"] = repo.RepoURL
		comp.Model.Version = version
		comp.Model.Name = repo.Name
		comp.Model.Category = v1alpha1.Category{
			Name: "",
		}
		comp.Model.DisplayName = manifests.FormatToReadableString(comp.Model.Name)
		components = append(components, comp)
	}
	return components, nil
}

func (repo *GithubRepo) UpdatePackageData() error {
	return nil
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
