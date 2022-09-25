package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
	"github.com/layer5io/meshkit/utils/component"
	"github.com/layer5io/meshkit/utils/manifests"
)

const ArtifactHubAPIEndpint = "https://artifacthub.io/api/v1"
const ArtifactHubChartUrlFieldName = "content_url"

// internal representation of artifacthub package
// it contains information we need to identify a package using ArtifactHub API
type AhPackage struct {
	Name              string
	Repository        string
	Organization      string
	Url               string
	Official          bool
	VerifiedPublisher bool
	Version           string
}

func (pkg AhPackage) GenerateComponents() ([]v1alpha1.Component, error) {
	components := make([]v1alpha1.Component, 0)

	// TODO: Move this to the configuration
	crds, err := manifests.GetCrdsFromHelm(pkg.Url)
	if err != nil {
		return components, ErrComponentGenerate(err)
	}
	for _, crd := range crds {
		comp, err := component.Generate(crd)
		if err != nil {
			continue
		}
		comp.Metadata["version"] = pkg.Version
		components = append(components, comp)
	}
	return components, nil
}

// function that will take the AhPackage as input and give the helm chart url for that package
func (pkg *AhPackage) UpdatePackageData() error {
	if pkg.Url != "" {
		return nil
	}
	url := fmt.Sprintf("%s/packages/helm/%s/%s", ArtifactHubAPIEndpint, pkg.Repository, pkg.Name)
	resp, err := http.Get(url)
	if err != nil {
		return ErrGetChartUrl(err)
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code %d for %s", resp.StatusCode, url)
		return ErrGetChartUrl(err)
	}
	defer resp.Body.Close()
	var res map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return ErrGetChartUrl(err)
	}
	chartUrl := res[ArtifactHubChartUrlFieldName].(string)
	official := res["repository"].(map[string]interface{})["official"].(bool)
	verPub := res["repository"].(map[string]interface{})["verified_publisher"].(bool)
	version := res["version"].(string)
	pkg.Url = chartUrl
	pkg.Official = official
	pkg.VerifiedPublisher = verPub
	pkg.Version = version
	return nil
}
