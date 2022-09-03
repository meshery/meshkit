package artifacthub

import (
	"github.com/layer5io/meshkit/errors"
)

var (
	ErrGetChartUrlCode  = "11092"
	ErrGetAhPackageCode = "1093"
)

func ErrGetChartUrl(err error) error {
	return errors.New(ErrGetChartUrlCode, errors.Alert, []string{"Could not get the chart url for this ArtifactHub package"}, []string{err.Error()}, []string{""}, []string{"make sure that the package exists"})
}

func ErrGetAhPackage(err error) error {
	return errors.New(ErrGetAhPackageCode, errors.Alert, []string{"Could not get the ArtifactHub package with the given name"}, []string{err.Error()}, []string{""}, []string{"make sure that the package exists"})
}
