package kubernetes

import (
	"fmt"
	"time"

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

	// ErrHelmChartFetchCode is the error raised when the Helm chart to apply could not be
	// located in its repository, downloaded, or loaded.
	ErrHelmChartFetchCode = "meshkit-11346"

	// ErrHelmClusterUnreachableCode is the error raised when Helm could not reach the
	// Kubernetes API server of the cluster the chart is applied to.
	ErrHelmClusterUnreachableCode = "meshkit-11347"

	// ErrHelmReleaseStorageCode is the error raised when Helm could not read or update the
	// stored record of a release (its history).
	ErrHelmReleaseStorageCode = "meshkit-11348"

	// ErrHelmReleasePendingCode is the error raised when the latest revision of a release is
	// pending-install, pending-upgrade or pending-rollback, and the operation that left it
	// there may still be running.
	ErrHelmReleasePendingCode = "meshkit-11349"

	// ErrHelmReleaseStateCode is the error raised when the latest revision of a release is in
	// a state that ApplyHelmChart does not install or upgrade over, such as uninstalling.
	ErrHelmReleaseStateCode = "meshkit-11350"
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

// ErrHelmChartFetch is returned when the Helm chart to apply could not be located in its
// repository, downloaded, or loaded, before anything was done to the cluster.
func ErrHelmChartFetch(err error) error {
	e := errors.NewV3(ErrHelmChartFetchCode, errors.Alert,
		[]string{"Could not fetch the Helm chart"},
		[]string{},
		[]errors.Cause{
			{ID: "chart-source-unreachable", Description: "The chart repository or chart URL could not be reached from this host.", Remediations: []errors.Remediation{
				{ID: "check-chart-source-access", Description: "Verify that this host can reach the chart repository or chart URL (network egress, proxy and DNS)."},
			}},
			{ID: "chart-version-not-published", Description: "The requested chart, chart version or app version is not published in the repository.", Remediations: []errors.Remediation{
				{ID: "check-chart-version", Description: "Verify that the chart and the requested version are listed in the repository's index.yaml."},
			}},
			{ID: "chart-archive-invalid", Description: "The chart archive is not a valid Helm chart.", Remediations: []errors.Remediation{
				{ID: "check-chart-archive", Description: "Inspect the archive with helm show chart and republish the chart if it is corrupt."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrHelmClusterUnreachable is returned when Helm could not reach the Kubernetes API server of
// the cluster that the release is applied to.
func ErrHelmClusterUnreachable(releaseName, namespace string, err error) error {
	e := errors.NewV3(ErrHelmClusterUnreachableCode, errors.Alert,
		[]string{"Helm could not reach the Kubernetes cluster"},
		[]string{},
		[]errors.Cause{
			{ID: "api-server-unreachable", Description: "The Kubernetes API server could not be reached.", Remediations: []errors.Remediation{
				{ID: "check-api-server-connectivity", Description: "Verify that the cluster is running and that its API server is reachable from this host (network, DNS and firewall)."},
			}},
			{ID: "credentials-rejected", Description: "The API server rejected the kubeconfig's credentials, for example because a token expired.", Remediations: []errors.Remediation{
				{ID: "refresh-kubeconfig-credentials", Description: "Refresh the credentials in the kubeconfig and connect to the cluster again."},
			}},
		})
	e.LongDescription = append(e.LongDescription, fmt.Sprintf("Cannot apply Helm release %q in namespace %q.", releaseName, namespace))
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrHelmReleaseStorage is returned when Helm could not read or update the stored record of a
// release: by default, the Secrets that hold its revisions in the release's namespace.
func ErrHelmReleaseStorage(releaseName, namespace string, err error) error {
	e := errors.NewV3(ErrHelmReleaseStorageCode, errors.Alert,
		[]string{"Could not read or update the Helm release record"},
		[]string{},
		[]errors.Cause{
			{ID: "release-storage-forbidden", Description: "The kubeconfig's identity is not allowed to read or write the Secrets in which Helm records the release in its namespace.", Remediations: []errors.Remediation{
				{ID: "grant-release-storage-access", Description: "Grant the identity get, list, create, update and delete on Secrets in the release's namespace."},
			}},
			{ID: "release-storage-unavailable", Description: "Helm's release storage could not be initialized or reached, for example the database of the SQL storage driver.", Remediations: []errors.Remediation{
				{ID: "check-release-storage", Description: "Verify the Helm storage driver configuration and, for the SQL driver, that its database is reachable."},
			}},
		})
	e.LongDescription = append(e.LongDescription, fmt.Sprintf("Helm release %q in namespace %q.", releaseName, namespace))
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrHelmReleasePending is returned when the latest revision of a release is pending-install,
// pending-upgrade or pending-rollback and ApplyHelmChart will not recover it: it has been pending
// for less than the pending timeout, so the operation that left it there may still be running, or
// the call is a dry run, which never changes the release record, or the recovery is turned off (a
// negative timeout).
func ErrHelmReleasePending(releaseName, namespace, status string, revision int, pendingFor, timeout time.Duration, dryRun bool) error {
	e := errors.NewV3(ErrHelmReleasePendingCode, errors.Alert,
		[]string{"Helm release has an operation in progress"},
		[]string{},
		[]errors.Cause{
			{ID: "operation-in-progress", Description: "Another install, upgrade or rollback of the release is still running, for example from another process or the Helm CLI.", Remediations: []errors.Remediation{
				{ID: "wait-and-retry", Description: "Wait for the running operation to finish, then retry."},
			}},
			{ID: "operation-interrupted", Description: "The process that ran the last install, upgrade or rollback stopped before finishing it, for example because it was restarted, and left the release pending.", Remediations: []errors.Remediation{
				{ID: "retry-after-pending-timeout", Description: "Retry once the release has been pending for longer than the pending timeout: the revision is then marked failed and the release upgraded, or installed again if no revision was ever deployed."},
				{ID: "recover-pending-release-manually", Description: "To recover it now, inspect it with helm history <release> -n <namespace>, then run helm rollback <release> <last deployed revision> -n <namespace>, or helm uninstall <release> -n <namespace> if no revision was ever deployed."},
			}},
		})
	e.LongDescription = append(e.LongDescription, fmt.Sprintf("Helm release %q in namespace %q has been %s at revision %d for %s.", releaseName, namespace, status, revision, pendingFor.Round(time.Second)))
	switch {
	case timeout < 0:
		e.LongDescription = append(e.LongDescription, "Recovery of abandoned operations is turned off for this call (a negative pending timeout), so the release is left for an operator to recover.")
	case dryRun && pendingFor >= timeout:
		e.LongDescription = append(e.LongDescription, fmt.Sprintf("That is longer than the %s pending timeout, so the operation is treated as abandoned, but a dry run never changes the release record; run without dry run to recover the release.", timeout))
	default:
		e.LongDescription = append(e.LongDescription, fmt.Sprintf("It is treated as abandoned once it has been pending for %s: the revision is then marked failed and the release upgraded, or installed again if no revision was ever deployed.", timeout))
	}
	return e
}

// ErrHelmReleaseState is returned when the latest revision of a release is in a state that
// ApplyHelmChart does not install or upgrade over: uninstalling, or a status it does not know.
func ErrHelmReleaseState(releaseName, namespace, status string, revision int) error {
	e := errors.NewV3(ErrHelmReleaseStateCode, errors.Alert,
		[]string{"Helm release is in a state that cannot be installed or upgraded"},
		[]string{},
		[]errors.Cause{
			{ID: "uninstall-not-finished", Description: "An uninstall of the release is still running or was interrupted, so the release is uninstalling.", Remediations: []errors.Remediation{
				{ID: "finish-uninstall", Description: "Wait for a running uninstall to finish, or complete an interrupted one with helm uninstall <release> -n <namespace>, then retry."},
			}},
			{ID: "unrecognized-release-status", Description: "The latest revision of the release has a status that this version of MeshKit does not handle.", Remediations: []errors.Remediation{
				{ID: "inspect-release-history", Description: "Inspect the release with helm history <release> -n <namespace>, resolve it with helm rollback or helm uninstall, then retry."},
			}},
		})
	e.LongDescription = append(e.LongDescription, fmt.Sprintf("Helm release %q in namespace %q is %s at revision %d.", releaseName, namespace, status, revision))
	return e
}

// instanceDescription returns the lines that describe this occurrence of err, for the long
// description of the error that reports it. For a MeshKit error that is its own long
// description, without the probable cause and remediation that its Error() appends, which
// belong to the inner error and would contradict the reporting error's own; for any other error
// it is the message.
func instanceDescription(err error) []string {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsErrorV2(err); ok {
		if lines := errors.NonEmptyLines(e.LongDescription); len(lines) > 0 {
			return lines
		}
	}
	return []string{err.Error()}
}
