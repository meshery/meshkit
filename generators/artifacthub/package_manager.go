package artifacthub

import (
	"fmt"
	"strings"

	"github.com/meshery/meshkit/generators/models"
)

type ArtifactHubPackageManager struct {
	PackageName string
	SourceURL   string
}

func (ahpm ArtifactHubPackageManager) GetPackage() (models.Package, error) {
	// Check if SourceURL is an actual URL (from a previous generation)
	// If so, create a package directly from it instead of searching
	if ahpm.SourceURL != "" && (strings.HasPrefix(ahpm.SourceURL, "http://") || strings.HasPrefix(ahpm.SourceURL, "https://") || strings.HasPrefix(ahpm.SourceURL, "oci://")) {
		// SourceURL is an actual URL, use it directly
		pkg := AhPackage{
			Name:     ahpm.PackageName,
			ChartUrl: ahpm.SourceURL,
		}
		// Try to extract version from the URL or fetch it
		_ = pkg.UpdatePackageData() // This might fail but that's okay, ChartUrl is already set
		return pkg, nil
	}
	
	// get relevant packages by searching with package name
	pkgs, err := GetAhPackagesWithName(ahpm.PackageName)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, ErrNoPackageFound(ahpm.PackageName, "Artifacthub")
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
