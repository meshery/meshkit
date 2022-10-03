package artifacthub

import (
	"fmt"

	"github.com/layer5io/meshkit/models"
)

type ArtifactHubPackageManager struct {
	PackageName string
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
	// filter only packages with crds
	pkgs = FilterPackagesWithCrds(pkgs)
	pkgs = SortPackagesWithScore(pkgs)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("could not find any appropriate artifacthub package")
	}
	return pkgs[0], nil
}
