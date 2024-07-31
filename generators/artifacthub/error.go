package artifacthub

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrGetChartUrlCode        = "meshkit-11134"
	ErrGetAhPackageCode       = "meshkit-11135"
	ErrGetAllHelmPackagesCode = "meshkit-11137"
	ErrChartUrlEmptyCode      = "meshkit-11245"
	ErrNoPackageFoundCode     = "meshkit-11246"
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

func ErrChartUrlEmpty(modelName string, registrantName string) error {
	return errors.New(
		ErrChartUrlEmptyCode,
		errors.Alert,
		[]string{fmt.Sprintf("The Chart URL for the %s model is empty.", modelName)},
		[]string{fmt.Sprintf("provided Chart URL for the model %s is empty.", modelName)},
		[]string{fmt.Sprintf("%s does not have Chart URL for %s", registrantName, modelName)},
		[]string{fmt.Sprintf("Please provide the Chart URL for the model %s.", modelName)},
	)
}
func ErrNoPackageFound(modelName string, registrantName string) error {
	return errors.New(
		ErrNoPackageFoundCode,
		errors.Alert,
		[]string{fmt.Sprintf("No package found for the model %s.", modelName)},
		[]string{fmt.Sprintf("there was no package for %s model.", modelName)},
		[]string{fmt.Sprintf("%s does not have any package for model name %s", registrantName, modelName)},
		[]string{fmt.Sprintf("Please provide the correct package name for the model %s.", modelName)},
	)
}
