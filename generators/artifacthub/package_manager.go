package artifacthub

import (
	"fmt"
	"net/url"

	"github.com/layer5io/meshkit/models"
)

type ArtifactHubPackageManager struct {
	PackageName string
	SourceURL   string
}

func (ahpm ArtifactHubPackageManager) GetPackage() (models.Package, error) {
	// get relevant packages
	pkgs, err := GetAhPackagesWithName(ahpm.PackageName)
	if err != nil {
		return nil, err
	}
	// update package information
	for i, ap := range pkgs {
		if ahpm.SourceURL != "" {
			url, err := url.Parse(ahpm.SourceURL)
			if err != nil {
				ap.ChartUrl = url.String()
				pkgs[i] = ap
				continue
			}
		}
		_ = ap.UpdatePackageData()
		pkgs[i] = ap
	}
	if ahpm.SourceURL != "" {
		pkgs = FilterPackageWithGivenSourceURL(pkgs, ahpm.SourceURL)
		if len(pkgs) != 0 {
			return pkgs[0], nil
		}
	}
	// Add filtering/sort based on preferred_models.yaml as well.
	pkgs = SortPackagesWithScore(pkgs)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("could not find any appropriate artifacthub package")
	}
	return pkgs[0], nil
}
