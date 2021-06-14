package kubernetes

import "github.com/layer5io/meshkit/errors"

var (
	ErrApplyManifestCode    = "11021"
	ErrServiceDiscoveryCode = "11022"
	ErrApplyHelmChartCode   = "11023"
	ErrNewKubeClientCode    = "11024"
	ErrNewDynClientCode     = "11025"
	ErrNewDiscoveryCode     = "11026"
	ErrNewInformerCode      = "11027"
	ErrEndpointNotFoundCode = "11028"
	ErrInvalidAPIServerCode = "11029"
	ErrLoadConfigCode       = "11030"
	ErrValidateConfigCode   = "11031"

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

// ErrLoadConfig is the error which occurs in the process of loading a kubernetes config
func ErrLoadConfig(err error) error {
	return errors.NewDefault(ErrLoadConfigCode, "Error loading kubernetes config", err.Error())
}

// ErrValidateConfig is the error which occurs in the process of validating a kubernetes config
func ErrValidateConfig(err error) error {
	return errors.NewDefault(ErrValidateConfigCode, "Validation failed in the kubernetes config", err.Error())
}
