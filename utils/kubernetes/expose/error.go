package expose

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ErrExposeResourceCode is generated while exposing the kubernetes resource
	ErrExposeResourceCode = "meshkit_test_code"

	// ErrGettingResourceCode is generated when there is an error getting the kubernetes resource
	ErrGettingResourceCode = "meshkit_test_code"

	// ErrTraverserCode is a collection of errors generated during traversing the resources
	ErrTraverserCode = "meshkit_test_code"

	// ErrResourceCannotBeExposedCode is generated when the given resource cannot be exposed
	ErrResourceCannotBeExposedCode = "meshkit_test_code"

	// ErrSelectorBasedMapCode is generated when the given resource's selectors can't
	// be parsed to a map
	ErrSelectorBasedMapCode = "meshkit_test_code"

	// ErrProtocolBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrProtocolBasedMapCode = "meshkit_test_code"

	// ErrLableBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrLableBasedMapCode = "meshkit_test_code"

	// ErrPortParsingCode is generated when the given resource's ports can't
	// be parsed to a slice
	ErrPortParsingCode = "meshkit_test_code"

	// ErrGenerateServiceCode is generated when a service cannot be generated
	// for the given resource
	ErrGenerateServiceCode = "meshkit_test_code"

	// ErrConstructingRestHelperCode is generated when a rest helper cannot be generated
	// for the generated service
	ErrConstructingRestHelperCode = "meshkit_test_code"

	// ErrCreatingServiceCode is generated when there is an error deploying the service
	ErrCreatingServiceCode = "meshkit_test_code"
)

// Custom errors and generators - These errors are not meshkit errors, they are intended to be wrapped
// with more common and generic meshkit errors

var (
	// ErrPodHasNoLabels is the error for pods with no labels
	ErrPodHasNoLabels = fmt.Errorf("the pod has no labels and cannot be exposed")

	// ErrServiceHasNoSelectors is the error for service with no selectors
	ErrServiceHasNoSelectors = fmt.Errorf("the service has no pod selector set")

	// ErrInvalidDeploymentNoSelectorsLabels is the error for deployment (v1beta1) with no selectors and labels
	ErrInvalidDeploymentNoSelectorsLabels = fmt.Errorf("the deployment has no labels or selectors and cannot be exposed")

	// ErrInvalidDeploymentNoSelectors is the error for deployment (v1) with no selectors
	ErrInvalidDeploymentNoSelectors = fmt.Errorf("invalid deployment: no selectors, therefore cannot be exposed")

	// ErrInvalidReplicaNoSelectorsLabels is the error for replicaset (v1beta1) with no selectors and labels
	ErrInvalidReplicaNoSelectorsLabels = fmt.Errorf("the replica set has no labels or selectors and cannot be exposed")

	// ErrInvalidReplicaSetNoSelectors is the error for replicaset (v1) with no selectors
	ErrInvalidReplicaSetNoSelectors = fmt.Errorf("invalid replicaset: no selectors, therefore cannot be exposed")

	// ErrNoPortsFoundForHeadlessResource is the error when no ports are found for non headless resource
	ErrNoPortsFoundForHeadlessResource = fmt.Errorf("no ports found for the non headless resource")
)

// ErrUnknownSessionAffinityErr is the error for unknown session affinity
func ErrUnknownSessionAffinityErr(sa SessionAffinity) error {
	return fmt.Errorf("unknown session affinity: %s", sa)
}

// ErrMatchExpressionsConvertionErr is the error for failed match expression conversion
func ErrMatchExpressionsConvertionErr(me []metav1.LabelSelectorRequirement) error {
	return fmt.Errorf("couldn't convert expressions - \"%+v\" to map-based selector format", me)
}

// ErrFailedToExtractPodSelectorErr is the error for failed to extract pod selector
func ErrFailedToExtractPodSelectorErr(object runtime.Object) error {
	return fmt.Errorf("cannot extract pod selector from %T", object)
}

// ErrFailedToExtractPorts is the error for failed to extract ports
func ErrFailedToExtractPorts(object runtime.Object) error {
	return fmt.Errorf("cannot extract ports from %T", object)
}

// ErrFailedToExtractProtocolsErr is the error for extracting ports
func ErrFailedToExtractProtocolsErr(object runtime.Object) error {
	return fmt.Errorf("cannot extract protocols from %T", object)
}

// ErrCannotExposeObjectErr is the error if the given object cannot be exposed
func ErrCannotExposeObjectErr(kind schema.GroupKind) error {
	return fmt.Errorf("cannot expose a %s", kind)
}

// Meshkit errors - These errors are intended to wrap more specific errors while maintaining
// a stack trace within the errors

// ErrExposeResource is the error when there is an error exposing the kubernetes resource
func ErrExposeResource(err error) error {
	return errors.NewDefault(ErrExposeResourceCode, err.Error())
}

// ErrGettingResource is the error when there is an error getting the kubernetes resource
func ErrGettingResource(err error) error {
	return errors.NewDefault(ErrGettingResourceCode, err.Error())
}

// ErrTraverser is the error is collection of error generated while traversing the resources
func ErrTraverser(err error) error {
	return errors.NewDefault(ErrTraverserCode, err.Error())
}

// ErrResourceCannotBeExposed is the error if the given resource cannot be exposed
func ErrResourceCannotBeExposed(err error, resourceKind string) error {
	return errors.NewDefault(
		ErrResourceCannotBeExposedCode,
		fmt.Sprintf("resource type %s cannot be exposed: %s", resourceKind, err.Error()),
	)
}

// ErrSelectorBasedMap is the error when the given resource's selectors can't
// be parsed to a map
func ErrSelectorBasedMap(err error) error {
	return errors.NewDefault(ErrSelectorBasedMapCode, err.Error())
}

// ErrProtocolBasedMap is the error when the given resource's protocols can't
// be parsed to a map
func ErrProtocolBasedMap(err error) error {
	return errors.NewDefault(ErrProtocolBasedMapCode, err.Error())
}

// ErrLabelBasedMap is the error when the given resource's labels can't
// be parsed to a map
func ErrLabelBasedMap(err error) error {
	return errors.NewDefault(ErrLableBasedMapCode, err.Error())
}

// ErrPortParsing is the error when the given resource's ports can't
// be parsed to a slice
func ErrPortParsing(err error) error {
	return errors.NewDefault(ErrPortParsingCode, err.Error())
}

// ErrGenerateService is the error when a service cannot be generated
// for the given resource
func ErrGenerateService(err error) error {
	return errors.NewDefault(ErrGenerateServiceCode, err.Error())
}

// ErrConstructingRestHelper is the error when a rest helper cannot be generated
// for the generated service
func ErrConstructingRestHelper(err error) error {
	return errors.NewDefault(ErrConstructingRestHelperCode, err.Error())
}

// ErrCreatingService is the error when there is an error deploying the service
func ErrCreatingService(err error) error {
	return errors.NewDefault(ErrCreatingServiceCode, err.Error())
}
