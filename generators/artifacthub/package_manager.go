package artifacthub

import (
	"fmt"
	"net/url"

	"github.com/layer5io/meshkit/generators/models"
)

type ArtifactHubPackageManager struct {
	PackageName string
	SourceURL   string
}

func (ahpm ArtifactHubPackageManager) GetPackage() (models.Package, error) {
	// Try to extract package name from URL if available
	searchName := ahpm.PackageName
	if ahpm.SourceURL != "" {
		// Try to parse the URL
		parsedURL, err := url.Parse(ahpm.SourceURL)
		if err == nil {
			// Extract the ts_query_web parameter if it exists
			queryParams := parsedURL.Query()
			if tsQueryWeb := queryParams.Get("ts_query_web"); tsQueryWeb != "" {
				searchName = tsQueryWeb
			}
		}
	}

	// get relevant packages with either the extracted name or original name
	pkgs, err := GetAhPackagesWithName(searchName)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, ErrNoPackageFound(searchName, "Artifacthub")
	}
	// update package information
	for i, ap := range pkgs {
		_ = ap.UpdatePackageData()
		pkgs[i] = ap
	}

	// Add filtering/sort based on preferred_models.yaml as well.
	pkgs = SortPackagesWithScore(pkgs)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("could not find any appropriate artifacthub package")
	}
	return pkgs[0], nil
}
