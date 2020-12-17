package kubernetes

import "github.com/layer5io/meshkit/errors"

func ErrApplyManifest(err error) error {
	return errors.NewDefault(errors.ErrApplyManifest, "Error Applying manifest: "+err.Error())
}

// ErrServiceDiscovery returns an error of type "ErrServiceDiscovery" along with the passed error
func ErrServiceDiscovery(err error) error {
	return errors.NewDefault(errors.ErrServiceDiscovery, "Error Discovering service: "+err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrApplyHelmChart(err error) error {
	return errors.NewDefault(errors.ErrApplyHelmChart, "Error applying helm chart: "+err.Error())
}
