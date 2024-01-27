package artifacthub

import (
	"fmt"

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
