package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/layer5io/meshkit/utils/manifests"
)

var AhApiSearchParams = map[string]string{
	"offset": "0",
	"limit":  "10",
	"facets": "false",
	"kind":   "0", // represents HELM types
	"sort":   "relevance",
}

const AhTextSearchQueryFieldName = "ts_query_web"

func GetAhPackagesWithName(name string) ([]AhPackage, error) {
	pkgs := make([]AhPackage, 0)
	url := fmt.Sprintf("%s/packages/search?%s=%s&", ArtifactHubAPIEndpint, AhTextSearchQueryFieldName, name)
	// add params
	for key, val := range AhApiSearchParams {
		url = fmt.Sprintf("%s%s=%s&", url, key, val)
	}
	// get packages
	resp, err := http.Get(url)
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code %d for %s", resp.StatusCode, url)
		return nil, ErrGetAhPackage(err)
	}
	defer resp.Body.Close()
	var res map[string]([]map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}
	resPkgs := res["packages"]
	for _, pkg := range resPkgs {
		pkgs = append(pkgs, *parseArtifacthubResponse(pkg))
	}
	return pkgs, nil
}

func FilterPackagesWithCrds(pkgs []AhPackage) []AhPackage {
	out := make([]AhPackage, 0)
	for _, ap := range pkgs {
		crds, err := manifests.GetCrdsFromHelm(ap.ChartUrl)
		if err == nil && len(crds) != 0 {
			out = append(out, ap)
		}
	}
	return out
}
