package kubernetes

import "github.com/layer5io/meshkit/errors"

var (
	ErrApplyManifestCode    = "test_code"
	ErrServiceDiscoveryCode = "test_code"
	ErrApplyHelmChartCode   = "test_code"
	ErrNewKubeClientCode    = "test_code"
	ErrNewDynClientCode     = "test_code"
	ErrNewDiscoveryCode     = "test_code"
	ErrNewInformerCode      = "test_code"
	ErrEndpointNotFoundCode = "test_code"
	ErrInvalidAPIServerCode = "test_code"

	ErrEndpointNotFound = errors.NewDefault(ErrEndpointNotFoundCode, "Unable to discover an endpoint")
	ErrInvalidAPIServer = errors.NewDefault(ErrInvalidAPIServerCode, "Invalid API Server URL")
)

func ErrApplyManifest(err error) error {
	return errors.NewDefault(ErrApplyManifestCode, "Error Applying manifest", err.Error())
}

// ErrServiceDiscovery returns an error of type "ErrServiceDiscovery" along with the passed error
func ErrServiceDiscovery(err error) error {
	return errors.NewDefault(ErrServiceDiscoveryCode, "Error Discovering service", err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrApplyHelmChart(err error) error {
	return errors.NewDefault(ErrApplyHelmChartCode, "Error applying helm chart", err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewKubeClient(err error) error {
	return errors.NewDefault(ErrNewKubeClientCode, "Error creating kubernetes clientset", err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewDynClient(err error) error {
	return errors.NewDefault(ErrNewDynClientCode, "Error creating dynamic client", err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewDiscovery(err error) error {
	return errors.NewDefault(ErrNewDiscoveryCode, "Error creating discovery client", err.Error())
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewInformer(err error) error {
	return errors.NewDefault(ErrNewInformerCode, "Error creating informer client", err.Error())
}
