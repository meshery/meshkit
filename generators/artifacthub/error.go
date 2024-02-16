package artifacthub

import (
	"github.com/layer5io/meshkit/errors"
)

var (
	ErrGetChartUrlCode        = "11092"
	ErrGetAhPackageCode       = "1093"
	ErrComponentGenerateCode  = "11094"
	ErrGetAllHelmPackagesCode = "11095"
)

func ErrGetAllHelmPackages(err error) error {
	return errors.New(ErrGetAllHelmPackagesCode, errors.Alert, []string{"Could not get HELM packages from Artifacthub"}, []string{err.Error()}, []string{""}, []string{"make sure that the artifacthub API service is available"})
}

func ErrGetChartUrl(err error) error {
	return errors.New(ErrGetChartUrlCode, errors.Alert, []string{"Could not get the chart url for this ArtifactHub package"}, []string{err.Error()}, []string{""}, []string{"make sure that the package exists"})
}

func ErrGetAhPackage(err error) error {
	return errors.New(ErrGetAhPackageCode, errors.Alert, []string{"Could not get the ArtifactHub package with the given name"}, []string{err.Error()}, []string{""}, []string{"make sure that the package exists"})
}

func ErrComponentGenerate(err error) error {
	return errors.New(ErrComponentGenerateCode, errors.Alert, []string{"failed to generate components for the package"}, []string{err.Error()}, []string{}, []string{"Make sure that the package is compatible"})
}
