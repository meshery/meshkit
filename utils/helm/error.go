package helm

import "github.com/layer5io/meshkit/errors"

var (
	ErrDryRunHelmChartCode = "replace_me"
	ErrLoadHelmChartCode       = "replace_me"
)

func ErrDryRunHelmChart(err error) error {
	return errors.New(ErrDryRunHelmChartCode, errors.Alert, []string{}, []string{err.Error()}, []string{}, []string{})
}

func ErrLoadHelmChart(err error) error {
	return errors.New(ErrLoadHelmChartCode, errors.Alert, []string{}, []string{err.Error()}, []string{}, []string{})
}
