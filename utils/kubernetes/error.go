package kubernetes

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

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
	// ErrCreatingHelmIndexCode represents the errors which are generated
	// during creation of helm index
	ErrCreatingHelmIndexCode = "11032"

	// ErrEntryWithAppVersionNotExistsCode represents the error which is generated
	// when no entry is found with specified name and app version
	ErrEntryWithAppVersionNotExistsCode = "11033"

	// ErrHelmRepositoryNotFoundCode represents the error which is generated when
	// no valid helm repository is found
	ErrHelmRepositoryNotFoundCode = "11034"

	// ErrEntryWithChartVersionNotExistsCode represents the error which is generated
	// when no entry is found with specified name and app version
	ErrEntryWithChartVersionNotExistsCode = "11036"
	ErrEndpointNotFound                   = errors.New(ErrEndpointNotFoundCode, errors.Alert, []string{"Unable to discover an endpoint"}, []string{}, []string{}, []string{})
	ErrInvalidAPIServer                   = errors.New(ErrInvalidAPIServerCode, errors.Alert, []string{"Invalid API Server URL"}, []string{}, []string{}, []string{})
)

func ErrApplyManifest(err error) error {
	return errors.New(ErrApplyManifestCode, errors.Alert, []string{"Error Applying manifest"}, []string{err.Error()}, []string{"Manifest could be invalid"}, []string{"Make sure manifest yaml is valid"})
}

// ErrServiceDiscovery returns an error of type "ErrServiceDiscovery" along with the passed error
func ErrServiceDiscovery(err error) error {
	return errors.New(ErrServiceDiscoveryCode, errors.Alert, []string{"Error Discovering service"}, []string{err.Error()}, []string{"Network not reachable to the service"}, []string{"Make sure the endpoint is reachable"})
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrApplyHelmChart(err error) error {
	return errors.New(ErrApplyHelmChartCode, errors.Alert, []string{"Error applying helm chart"}, []string{err.Error()}, []string{"Chart could be invalid"}, []string{"Make sure to apply valid chart"})
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewKubeClient(err error) error {
	return errors.New(ErrNewKubeClientCode, errors.Alert, []string{"Error creating kubernetes clientset"}, []string{err.Error()}, []string{"Kubernetes config is not accessible to meshery or not valid"}, []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"})
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewDynClient(err error) error {
	return errors.New(ErrNewDynClientCode, errors.Alert, []string{"Error creating dynamic client"}, []string{err.Error()}, []string{"Kubernetes config is not accessible to meshery or not valid"}, []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"})
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewDiscovery(err error) error {
	return errors.New(ErrNewDiscoveryCode, errors.Alert, []string{"Error creating discovery client"}, []string{err.Error()}, []string{"Discovery resource is invalid or doesnt exist"}, []string{"Makes sure the you input valid resource for discovery"})
}

// ErrApplyHelmChart is the error which occurs in the process of applying helm chart
func ErrNewInformer(err error) error {
	return errors.New(ErrNewInformerCode, errors.Alert, []string{"Error creating informer client"}, []string{err.Error()}, []string{"Informer is invalid or doesnt exist"}, []string{"Makes sure the you input valid resource for the informer"})
}

// ErrLoadConfig is the error which occurs in the process of loading a kubernetes config
func ErrLoadConfig(err error) error {
	return errors.New(ErrLoadConfigCode, errors.Alert, []string{"Error loading kubernetes config"}, []string{err.Error()}, []string{"Kubernetes config is not accessible to meshery or not valid"}, []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"})
}

// ErrValidateConfig is the error which occurs in the process of validating a kubernetes config
func ErrValidateConfig(err error) error {
	return errors.New(ErrValidateConfigCode, errors.Alert, []string{"Validation failed in the kubernetes config"}, []string{err.Error()}, []string{"Kubernetes config is not accessible to meshery or not valid"}, []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"})
}

// ErrCreatingHelmIndex is the error for creating helm index
func ErrCreatingHelmIndex(err error) error {
	return errors.New(ErrCreatingHelmIndexCode, errors.Alert, []string{"Error while creating Helm Index"}, []string{err.Error()}, []string{}, []string{})
}

// ErrEntryWithAppVersionNotExists is the error when an entry with the given app version is not found
func ErrEntryWithAppVersionNotExists(entry, appVersion string) error {
	return errors.New(ErrEntryWithAppVersionNotExistsCode, errors.Alert, []string{"Entry for the app version does not exist"}, []string{fmt.Sprintf("entry %s with app version %s does not exists", entry, appVersion)}, []string{}, []string{})
}

// ErrEntryWithChartVersionNotExists is the error when an entry with the given chart version is not found
func ErrEntryWithChartVersionNotExists(entry, appVersion string) error {
	return errors.New(ErrEntryWithChartVersionNotExistsCode, errors.Alert, []string{"Entry for the chart version does not exist"}, []string{fmt.Sprintf("entry %s with chart version %s does not exists", entry, appVersion)}, []string{}, []string{})
}

// ErrHelmRepositoryNotFound is the error when no valid remote helm repository is found
func ErrHelmRepositoryNotFound(repo string, err error) error {
	return errors.New(ErrHelmRepositoryNotFoundCode, errors.Alert, []string{"Helm repo not found"}, []string{fmt.Sprintf("either the repo %s does not exists or is corrupt: %v", repo, err)}, []string{}, []string{})
}
