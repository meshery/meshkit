package helm

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrDryRunHelmChartCode = "11115"
	ErrLoadHelmChartCode   = "11116"
)

func ErrDryRunHelmChart(err error, chartName string) error {
	return errors.New(ErrDryRunHelmChartCode, errors.Alert, []string{fmt.Sprintf("error dry running helm chart %s", chartName)}, []string{err.Error()}, []string{"the chart is corrupted", "template structure is not valid"}, []string{"delete the chart and try again", "validate the chart and try again"})
}

func ErrLoadHelmChart(err error, path string) error {
	return errors.New(ErrLoadHelmChartCode, errors.Alert, []string{fmt.Sprintf("error loading helm chart at %s", path)}, []string{err.Error()}, []string{fmt.Sprintf("chart does not exist at the specified path %s", path), "chart might have been deleted", "insufficient permissions to read the chart"}, []string{"provide correct path to the chart directory/file", "ensure sufficient/correct permission to the chart directory/file"})
}
