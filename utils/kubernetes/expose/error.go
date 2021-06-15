package expose

import (
	"github.com/layer5io/meshkit/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ErrExposeResourceCode is generated while exposing the kubernetes resource
	ErrExposeResourceCode = "11032"

	// ErrGettingResourceCode is generated when there is an error getting the kubernetes resource
	ErrGettingResourceCode = "11033"

	// ErrTraverserCode is a collection of errors generated during traversing the resources
	ErrTraverserCode = "11034"

	// ErrResourceCannotBeExposedCode is generated when the given resource cannot be exposed
	ErrResourceCannotBeExposedCode = "11035"

	// ErrSelectorBasedMapCode is generated when the given resource's selectors can't
	// be parsed to a map
	ErrSelectorBasedMapCode = "11036"

	// ErrProtocolBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrProtocolBasedMapCode = "11037"

	// ErrLableBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrLableBasedMapCode = "11038"

	// ErrPortParsingCode is generated when the given resource's ports can't
	// be parsed to a slice
	ErrPortParsingCode = "11039"

	// ErrGenerateServiceCode is generated when a service cannot be generated
	// for the given resource
	ErrGenerateServiceCode = "11040"

	// ErrConstructingRestHelperCode is generated when a rest helper cannot be generated
	// for the generated service
	ErrConstructingRestHelperCode = "11041"

	// ErrCreatingServiceCode is generated when there is an error deploying the service
	ErrCreatingServiceCode = "11042"

	// ErrPodHasNoLabelsCode is generated when there is an error
	ErrPodHasNoLabelsCode = "11058"

	// ErrServiceHasNoSelectorsCode is generated when there is an error
	ErrServiceHasNoSelectorsCode = "11059"

	// ErrInvalidDeploymentNoSelectorsLabelsCode is generated when there is an error
	ErrInvalidDeploymentNoSelectorsLabelsCode = "11060"

	// ErrInvalidDeploymentNoSelectorsCode is generated when there is an error
	ErrInvalidDeploymentNoSelectorsCode = "11061"

	// ErrInvalidReplicaNoSelectorsLabelsCode is generated when there is an error
	ErrInvalidReplicaNoSelectorsLabelsCode = "11062"

	// ErrInvalidReplicaSetNoSelectorsCode is generated when there is an error
	ErrInvalidReplicaSetNoSelectorsCode = "11063"

	// ErrNoPortsFoundForHeadlessResourceCode is generated when there is an error
	ErrNoPortsFoundForHeadlessResourceCode = "11064"

	// ErrUnknownSessionAffinityErrCode is generated when there is an error
	ErrUnknownSessionAffinityErrCode = "11065"

	// ErrMatchExpressionsConvertionErrCode is generated when there is an error
	ErrMatchExpressionsConvertionErrCode = "11066"

	// ErrFailedToExtractPodSelectorErrCode is generated when there is an error
	ErrFailedToExtractPodSelectorErrCode = "11067"

	// ErrFailedToExtractPortsCode is generated when there is an error
	ErrFailedToExtractPortsCode = "11068"

	// ErrFailedToExtractProtocolsErrCode is generated when there is an error
	ErrFailedToExtractProtocolsErrCode = "11069"

	// ErrCannotExposeObjectErrCode is generated when there is an error
	ErrCannotExposeObjectErrCode = "11070"
)

// Custom errors and generators - These errors are not meshkit errors, they are intended to be wrapped
// with more common and generic meshkit errors

var (
	// ErrPodHasNoLabels is the error for pods with no labels
	ErrPodHasNoLabels = errors.New(ErrPodHasNoLabelsCode, errors.Alert, []string{"the pod has no labels and cannot be exposed"}, []string{}, []string{}, []string{})

	// ErrServiceHasNoSelectors is the error for service with no selectors
	ErrServiceHasNoSelectors = errors.New(ErrServiceHasNoSelectorsCode, errors.Alert, []string{"the service has no pod selector set"}, []string{}, []string{}, []string{})

	// ErrInvalidDeploymentNoSelectorsLabels is the error for deployment (v1beta1) with no selectors and labels
	ErrInvalidDeploymentNoSelectorsLabels = errors.New(ErrInvalidDeploymentNoSelectorsLabelsCode, errors.Alert, []string{"the deployment has no labels or selectors and cannot be exposed"}, []string{}, []string{}, []string{})

	// ErrInvalidDeploymentNoSelectors is the error for deployment (v1) with no selectors
	ErrInvalidDeploymentNoSelectors = errors.New(ErrInvalidDeploymentNoSelectorsCode, errors.Alert, []string{"invalid deployment: no selectors, therefore cannot be exposed"}, []string{}, []string{}, []string{})

	// ErrInvalidReplicaNoSelectorsLabels is the error for replicaset (v1beta1) with no selectors and labels
	ErrInvalidReplicaNoSelectorsLabels = errors.New(ErrInvalidReplicaNoSelectorsLabelsCode, errors.Alert, []string{"the replica set has no labels or selectors and cannot be exposed"}, []string{}, []string{}, []string{})

	// ErrInvalidReplicaSetNoSelectors is the error for replicaset (v1) with no selectors
	ErrInvalidReplicaSetNoSelectors = errors.New(ErrInvalidReplicaSetNoSelectorsCode, errors.Alert, []string{"invalid replicaset: no selectors, therefore cannot be exposed"}, []string{}, []string{}, []string{})

	// ErrNoPortsFoundForHeadlessResource is the error when no ports are found for non headless resource
	ErrNoPortsFoundForHeadlessResource = errors.New(ErrNoPortsFoundForHeadlessResourceCode, errors.Alert, []string{"no ports found for the non headless resource"}, []string{}, []string{}, []string{})
)

// ErrUnknownSessionAffinityErr is the error for unknown session affinity
func ErrUnknownSessionAffinityErr(sa SessionAffinity) error {
	return errors.New(ErrUnknownSessionAffinityErrCode, errors.Alert, []string{"unknown session affinity:", string(sa)}, []string{}, []string{}, []string{})
}

// ErrMatchExpressionsConvertionErr is the error for failed match expression conversion
func ErrMatchExpressionsConvertionErr(me []metav1.LabelSelectorRequirement) error {
	return errors.New(ErrMatchExpressionsConvertionErrCode, errors.Alert, []string{"couldn't convert expressions - to map-based selector format"}, []string{}, []string{}, []string{})
}

// ErrFailedToExtractPodSelectorErr is the error for failed to extract pod selector
func ErrFailedToExtractPodSelectorErr(object runtime.Object) error {
	return errors.New(ErrFailedToExtractPodSelectorErrCode, errors.Alert, []string{"cannot extract pod selector from ", object.GetObjectKind().GroupVersionKind().Kind}, []string{}, []string{}, []string{})
}

// ErrFailedToExtractPorts is the error for failed to extract ports
func ErrFailedToExtractPorts(object runtime.Object) error {
	return errors.New(ErrFailedToExtractPortsCode, errors.Alert, []string{"cannot extract ports from ", object.GetObjectKind().GroupVersionKind().Kind}, []string{}, []string{}, []string{})
}

// ErrFailedToExtractProtocolsErr is the error for extracting ports
func ErrFailedToExtractProtocolsErr(object runtime.Object) error {
	return errors.New(ErrFailedToExtractProtocolsErrCode, errors.Alert, []string{"cannot extract protocols from ", object.GetObjectKind().GroupVersionKind().Kind}, []string{}, []string{}, []string{})
}

// ErrCannotExposeObjectErr is the error if the given object cannot be exposed
func ErrCannotExposeObjectErr(kind schema.GroupKind) error {
	return errors.New(ErrCannotExposeObjectErrCode, errors.Alert, []string{"cannot expose a ", kind.String()}, []string{}, []string{}, []string{})
}

// Meshkit errors - These errors are intended to wrap more specific errors while maintaining
// a stack trace within the errors

// ErrExposeResource is the error when there is an error exposing the kubernetes resource
func ErrExposeResource(err error) error {
	return errors.New(ErrExposeResourceCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrGettingResource is the error when there is an error getting the kubernetes resource
func ErrGettingResource(err error) error {
	return errors.New(ErrGettingResourceCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrTraverser is the error is collection of error generated while traversing the resources
func ErrTraverser(err error) error {
	return errors.New(ErrTraverserCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrResourceCannotBeExposed is the error if the given resource cannot be exposed
func ErrResourceCannotBeExposed(err error, resourceKind string) error {
	return errors.New(ErrResourceCannotBeExposedCode,
		errors.Alert, []string{"resource type %s cannot be exposed: ", resourceKind}, []string{err.Error()}, []string{}, []string{})
}

// ErrSelectorBasedMap is the error when the given resource's selectors can't
// be parsed to a map
func ErrSelectorBasedMap(err error) error {
	return errors.New(ErrSelectorBasedMapCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrProtocolBasedMap is the error when the given resource's protocols can't
// be parsed to a map
func ErrProtocolBasedMap(err error) error {
	return errors.New(ErrProtocolBasedMapCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrLabelBasedMap is the error when the given resource's labels can't
// be parsed to a map
func ErrLabelBasedMap(err error) error {
	return errors.New(ErrLableBasedMapCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrPortParsing is the error when the given resource's ports can't
// be parsed to a slice
func ErrPortParsing(err error) error {
	return errors.New(ErrPortParsingCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrGenerateService is the error when a service cannot be generated
// for the given resource
func ErrGenerateService(err error) error {
	return errors.New(ErrGenerateServiceCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrConstructingRestHelper is the error when a rest helper cannot be generated
// for the generated service
func ErrConstructingRestHelper(err error) error {
	return errors.New(ErrConstructingRestHelperCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}

// ErrCreatingService is the error when there is an error deploying the service
func ErrCreatingService(err error) error {
	return errors.New(ErrCreatingServiceCode, errors.Alert, []string{err.Error()}, []string{}, []string{}, []string{})
}
