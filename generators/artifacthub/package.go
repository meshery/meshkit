package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
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
	Name              string `yaml:"name"`
	Repository        string `yaml:"repository"`
	Organization      string `yaml:"organization"`
	RepoUrl           string `yaml:"repo_url"`
	ChartUrl          string `yaml:"chart_url"`
	Official          bool   `yaml:"official"`
	VerifiedPublisher bool   `yaml:"verified_publisher"`
	CNCF              bool   `yaml:"cncf"`
	Version           string `yaml:"version"`
}

func (pkg AhPackage) GetVersion() string {
	return pkg.Version
}

func (pkg AhPackage) GenerateComponents() ([]v1beta1.ComponentDefinition, error) {
	components := make([]v1beta1.ComponentDefinition, 0)
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
		if comp.Model.Metadata == nil {
			comp.Model.Metadata = make(map[string]interface{})
		}
		comp.Model.Metadata["source_uri"] = pkg.ChartUrl
		comp.Model.Version = pkg.Version
		comp.Model.Name = pkg.Name
		comp.Model.Category = v1beta1.Category{
			Name: "",
		}
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
	if !strings.HasPrefix(chartUrl, "http") {
		if !strings.HasSuffix(pkg.RepoUrl, "/") {
			pkg.RepoUrl = pkg.RepoUrl + "/"
		}
		chartUrl = fmt.Sprintf("%s%s", pkg.RepoUrl, chartUrl)
	}
	pkg.ChartUrl = chartUrl
	return nil
}

func (pkg *AhPackage) Validator() {

}

// GetAllAhHelmPackages returns a list of all AhPackages and is super slow to avoid rate limits.
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
		name := p["name"].(string)
		repo := p["repository"].(map[string]interface{})["name"].(string)
		url := fmt.Sprintf("https://artifacthub.io/api/v1/packages/helm/%s/%s", repo, name)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if resp.StatusCode != 200 {
			err = fmt.Errorf("status code %d for %s", resp.StatusCode, url)
			fmt.Println(err)
			continue
		}
		defer resp.Body.Close()
		var res map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			fmt.Println(err)
			continue
		}

		pkgs = append(pkgs, *parseArtifacthubResponse(res))
		time.Sleep(500 * time.Millisecond)
	}
	return pkgs, nil
}

func parseArtifacthubResponse(response map[string]interface{}) *AhPackage {
	verified := false
	cncf := false
	official := false
	name, _ := response["name"].(string)
	version, _ := response["version"].(string)
	repository, ok := response["repository"].(map[string]interface{})
	var repoName, repoURL string

	if ok {
		if repository["name"] != nil {
			repoName = repository["name"].(string)
		}
		if repository["verified_publisher"] != nil {
			verified = repository["verified_publisher"].(bool)
		}
		if repository["official"] != nil {
			official = repository["official"].(bool)
		}
		if repository["cncf"] != nil {
			cncf = response["repository"].(map[string]interface{})["cncf"].(bool)
		}

		if repository["url"] != nil {
			repoURL = repository["url"].(string)
		}
	}

	return &AhPackage{
		Name:              name,
		Version:           version,
		Repository:        repoName,
		RepoUrl:           repoURL,
		VerifiedPublisher: verified,
		CNCF:              cncf,
		Official:          official,
	}
}
