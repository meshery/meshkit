package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/component"
	"github.com/layer5io/meshkit/utils/manifests"
	"gopkg.in/yaml.v2"
)

const ArtifactHubAPIEndpint = "https://artifacthub.io/api/v1"
const ArtifactHubChartUrlFieldName = "content_url"
const AhHelmExporterEndpoint = ArtifactHubAPIEndpint + "/helm-exporter"

// internal representation of artifacthub package
// it contains information we need to identify a package using ArtifactHub API
type AhPackage struct {
	Name              string
	Repository        string
	Organization      string
	RepoUrl           string
	ChartUrl          string
	Official          bool
	VerifiedPublisher bool
	Version           string
}

func (pkg AhPackage) GenerateComponents() ([]v1alpha1.ComponentDefinition, error) {
	components := make([]v1alpha1.ComponentDefinition, 0)
	// TODO: Move this to the configuration
	crds, err := manifests.GetCrdsFromHelm(pkg.ChartUrl)
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
		comp.Model.Version = pkg.Version
		comp.Model.Name = pkg.Repository
		comp.Model.DisplayName = manifests.FormatToReadableString(comp.Model.Name)
		components = append(components, comp)
	}
	return components, nil
}

// function that will take the AhPackage as input and give the helm chart url for that package
func (pkg *AhPackage) UpdatePackageData() error {
	if pkg.ChartUrl != "" {
		return nil
	}
	urlSuffix := "/index.yaml"
	if strings.HasSuffix(pkg.RepoUrl, "/") {
		urlSuffix = "index.yaml"
	}
	charts, err := utils.ReadRemoteFile(pkg.RepoUrl + urlSuffix)
	if err != nil {
		fmt.Println("bruh: ", err.Error())
		return ErrGetChartUrl(err)
	}
	var out map[string]interface{}
	err = yaml.Unmarshal([]byte(charts), &out)
	if err != nil {
		return ErrGetChartUrl(err)
	}
	entries, ok := out["entries"].(map[interface{}]interface{})
	if entries == nil || !ok {
		return ErrGetChartUrl(fmt.Errorf("Cannot extract chartUrl from repository helm index"))
	}
	pkgEntry, ok := entries[pkg.Name]
	if pkgEntry == nil || !ok {
		return ErrGetChartUrl(fmt.Errorf("Cannot extract chartUrl from repository helm index"))
	}
	urls, ok := pkgEntry.([]interface{})[0].(map[interface{}]interface{})["urls"]
	if urls == nil || !ok {
		return ErrGetChartUrl(fmt.Errorf("Cannot extract chartUrl from repository helm index"))
	}
	chartUrl, ok := urls.([]interface{})[0].(string)
	if !ok || chartUrl == "" {
		return ErrGetChartUrl(fmt.Errorf("Cannot extract chartUrl from repository helm index"))
	}
	pkg.ChartUrl = chartUrl
	return nil
}

func GetAllAhHelmPackages() ([]AhPackage, error) {
	pkgs := make([]AhPackage, 0)
	resp, err := http.Get(AhHelmExporterEndpoint)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code %d for %s", resp.StatusCode, AhHelmExporterEndpoint)
		return nil, ErrGetAllHelmPackages(err)
	}
	defer resp.Body.Close()
	var res []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	for _, p := range res {
		pkgs = append(pkgs, AhPackage{
			Name:       p["name"].(string),
			Version:    p["version"].(string),
			Repository: p["repository"].(map[string]interface{})["name"].(string),
			RepoUrl:    p["repository"].(map[string]interface{})["url"].(string),
		})
	}
	return pkgs, nil
}
