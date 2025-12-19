package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/component"
	"github.com/meshery/meshkit/utils/manifests"
	"github.com/meshery/schemas/models/v1beta1/category"
	_component "github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
	"gopkg.in/yaml.v2"
)

const ArtifactHubAPIEndpoint = "https://artifacthub.io/api/v1"
const ArtifactHubChartUrlFieldName = "content_url"
const AhHelmExporterEndpoint = ArtifactHubAPIEndpoint + "/helm-exporter"

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

func (pkg AhPackage) GetSourceURL() string {
	return pkg.ChartUrl
}

func (pkg AhPackage) GetName() string {
	return pkg.Name
}

func (pkg AhPackage) GenerateComponents(group string) ([]_component.ComponentDefinition, error) {
	components := make([]_component.ComponentDefinition, 0)
	// TODO: Move this to the configuration

	if pkg.ChartUrl == "" {
		fmt.Printf("WARN: Skipping package %q due to empty chart URL\n", pkg.Name)
		return components, nil
	}
	if strings.HasPrefix(pkg.ChartUrl, "oci://") {
		// Skip OCI charts for now - return empty components
		// TODO: Implement OCI chart support
		return components, nil
	}
	crds, err := manifests.GetCrdsFromHelm(pkg.ChartUrl)
	if err != nil {
		return components, ErrComponentGenerate(err)
	}
	for _, crd := range crds {
		comp, err := component.Generate(crd)
		if err != nil {
			continue
		}
		if comp.Model == nil {
			comp.Model = &model.ModelDefinition{}
		}
		if comp.Model.Metadata == nil {
			comp.Model.Metadata = &model.ModelDefinition_Metadata{}
		}

		if comp.Model.Metadata.AdditionalProperties == nil {
			comp.Model.Metadata.AdditionalProperties = make(map[string]interface{})
		}
		comp.Model.Metadata.AdditionalProperties["source_uri"] = pkg.ChartUrl
		comp.Model.Version = pkg.Version
		comp.Model.Name = pkg.Name
		comp.Model.Category = category.CategoryDefinition{
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

	if strings.HasPrefix(chartUrl, "http") || strings.HasPrefix(chartUrl, "oci://") {
		// URL is already complete (HTTP/HTTPS or OCI)
		pkg.ChartUrl = chartUrl
	} else {
		// Relative URL, prepend the repository URL
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

// GetAllAhHelmPackages returns a list of all AhPackages, using exponential backoff to handle rate limits.
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

		var resp *http.Response
		var err error

		for i := 0; i < 5; i++ {
			resp, err = http.Get(url)
			if err != nil {
				break
			}
			if resp.StatusCode == 429 {
				resp.Body.Close()
				time.Sleep(time.Duration(1<<i) * time.Second)
				continue
			}
			break
		}

		if err != nil {
			fmt.Println(err)
			continue
		}

		if resp.StatusCode != 200 {
			err = fmt.Errorf("status code %d for %s", resp.StatusCode, url)
			fmt.Println(err)
			resp.Body.Close()
			continue
		}

		var pkgRes map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&pkgRes)
		resp.Body.Close()

		if err != nil {
			fmt.Println(err)
			continue
		}

		parsedPkg := parseArtifacthubResponse(pkgRes)
		if parsedPkg != nil {
			pkgs = append(pkgs, *parsedPkg)
		}
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
