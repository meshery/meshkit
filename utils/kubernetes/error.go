package kubernetes

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
)

var (
	ErrApplyManifestCode    = "meshkit-11190"
	ErrServiceDiscoveryCode = "meshkit-11191"
	ErrApplyHelmChartCode   = "meshkit-11192"
	ErrNewKubeClientCode    = "meshkit-11193"
	ErrNewDynClientCode     = "meshkit-11194"
	ErrNewDiscoveryCode     = "meshkit-11195"
	ErrNewInformerCode      = "meshkit-11196"
	ErrEndpointNotFoundCode = "meshkit-11197"
	ErrInvalidAPIServerCode = "meshkit-11198"
	ErrLoadConfigCode       = "meshkit-11199"
	ErrValidateConfigCode   = "meshkit-11200"
	// ErrCreatingHelmIndexCode represents the errors which are generated
	// during creation of helm index
	ErrCreatingHelmIndexCode = "meshkit-11201"

	// ErrEntryWithAppVersionNotExistsCode represents the error which is generated
	// when no entry is found with specified name and app version
	ErrEntryWithAppVersionNotExistsCode = "meshkit-11202"

	// ErrHelmRepositoryNotFoundCode represents the error which is generated when
	// no valid helm repository is found
	ErrHelmRepositoryNotFoundCode = "meshkit-11203"

	// ErrEntryWithChartVersionNotExistsCode represents the error which is generated
	// when no entry is found with specified name and app version
	ErrEntryWithChartVersionNotExistsCode = "meshkit-11204"
	ErrEndpointNotFound                   = errors.New(ErrEndpointNotFoundCode, errors.Alert, []string{"Unable to discover an endpoint"}, []string{}, []string{}, []string{})
	ErrInvalidAPIServer                   = errors.New(ErrInvalidAPIServerCode, errors.Alert, []string{"Invalid API Server URL"}, []string{}, []string{}, []string{})
	ErrRestConfigFromKubeConfigCode       = "meshkit-11205"
)

func ErrApplyManifest(err error) error {
	short := []string{"Error Applying manifest"}
	long := []string{err.Error()}
	probable := []string{"Manifest could be invalid"}
	remedy := []string{"Make sure manifest yaml is valid"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrApplyManifestCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrServiceDiscovery handles service discovery errors, including Kubernetes status errors
func ErrServiceDiscovery(err error) error {
	// Define default error messages
	short := []string{"Error discovering service"}
	long := []string{err.Error()}
	probable := []string{"Network not reachable to the service"}
	remedy := []string{"Make sure the endpoint is reachable"}

	// If this is a Kubernetes status error, get more specific error details
	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrServiceDiscoveryCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrApplyHelmChart handles helm chart application errors, including Kubernetes status errors
func ErrApplyHelmChart(err error) error {
	short := []string{"Error applying helm chart"}
	long := []string{err.Error()}
	probable := []string{"Chart could be invalid"}
	remedy := []string{"Make sure to apply valid chart"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrApplyHelmChartCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrNewKubeClient handles Kubernetes client creation errors
func ErrNewKubeClient(err error) error {
	short := []string{"Error creating kubernetes clientset"}
	long := []string{err.Error()}
	probable := []string{"Kubernetes config is not accessible to meshery or not valid"}
	remedy := []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrNewKubeClientCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrNewDynClient handles dynamic client creation errors
func ErrNewDynClient(err error) error {
	short := []string{"Error creating dynamic client"}
	long := []string{err.Error()}
	probable := []string{"Kubernetes config is not accessible to meshery or not valid"}
	remedy := []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrNewDynClientCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrNewDiscovery handles discovery client creation errors
func ErrNewDiscovery(err error) error {
	short := []string{"Error creating discovery client"}
	long := []string{err.Error()}
	probable := []string{"Discovery resource is invalid or doesnt exist"}
	remedy := []string{"Makes sure the you input valid resource for discovery"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrNewDiscoveryCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrNewInformer handles informer creation errors
func ErrNewInformer(err error) error {
	short := []string{"Error creating informer client"}
	long := []string{err.Error()}
	probable := []string{"Informer is invalid or doesnt exist"}
	remedy := []string{"Makes sure the you input valid resource for the informer"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrNewInformerCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrLoadConfig handles config loading errors
func ErrLoadConfig(err error) error {
	short := []string{"Error loading kubernetes config"}
	long := []string{err.Error()}
	probable := []string{"Kubernetes config is not accessible to meshery or not valid"}
	remedy := []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrLoadConfigCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
}

// ErrValidateConfig handles config validation errors
func ErrValidateConfig(err error) error {
	short := []string{"Validation failed in the kubernetes config"}
	long := []string{err.Error()}
	probable := []string{"Kubernetes config is not accessible to meshery or not valid"}
	remedy := []string{"Upload your kubernetes config via the settings dashboard. If uploaded, wait for a minute for it to get initialized"}

	if utils.IsErrKubeStatusErr(err) {
		statusErr := err.(*kubeerror.StatusError)
		short, long, probable, remedy = utils.ParseKubeStatusErr(statusErr)
	}

	return errors.New(ErrValidateConfigCode, errors.Alert,
		short,
		long,
		probable,
		remedy)
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

// ErrRestConfigFromKubeConfig returns an error when failing to create a REST config from a kubeconfig file.
func ErrRestConfigFromKubeConfig(err error) error {
	return errors.New(ErrRestConfigFromKubeConfigCode,
		errors.Alert,
		[]string{"Failed to create REST config from kubeconfig."},
		[]string{fmt.Sprintf("Error occured while creating REST config from kubeconfig: %s", err.Error())},
		[]string{
			"The provided kubeconfig data might be invalid or corrupted.",
			"The kubeconfig might be incomplete or missing required fields."},
		[]string{
			"Verify that the kubeconfig data is valid.",
			"Ensure the kubeconfig contains all necessary cluster, user, and context information.",
			"Check if the kubeconfig data was properly read and passed to the function."},
	)
}
