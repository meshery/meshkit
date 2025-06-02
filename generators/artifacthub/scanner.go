package artifacthub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/meshery/meshkit/utils/manifests"
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

	// Construct URL with encoded query parameters
	baseURL, err := url.Parse(fmt.Sprintf("%s/packages/search", ArtifactHubAPIEndpoint))
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}

	query := url.Values{}
	query.Add(AhTextSearchQueryFieldName, name)

	for key, val := range AhApiSearchParams {
		query.Add(key, val)
	}

	baseURL.RawQuery = query.Encode()
	finalURL := baseURL.String()

	// Get packages
	resp, err := http.Get(finalURL)
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code %d for %s", resp.StatusCode, finalURL)
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
