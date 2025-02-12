package expose

import (
	"github.com/layer5io/meshkit/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ErrExposeResourceCode is generated while exposing the kubernetes resource
	ErrExposeResourceCode = "meshkit-11205"

	// ErrGettingResourceCode is generated when there is an error getting the kubernetes resource
	ErrGettingResourceCode = "meshkit-11206"

	// ErrTraverserCode is a collection of errors generated during traversing the resources
	ErrTraverserCode = "meshkit-11207"

	// ErrResourceCannotBeExposedCode is generated when the given resource cannot be exposed
	ErrResourceCannotBeExposedCode = "meshkit-11208"

	// ErrSelectorBasedMapCode is generated when the given resource's selectors can't
	// be parsed to a map
	ErrSelectorBasedMapCode = "meshkit-11209"

	// ErrProtocolBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrProtocolBasedMapCode = "meshkit-11210"

	// ErrLableBasedMapCode is generated when the given resource's protocol can't
	// be parsed to a map
	ErrLableBasedMapCode = "meshkit-11211"

	// ErrPortParsingCode is generated when the given resource's ports can't
	// be parsed to a slice
	ErrPortParsingCode = "meshkit-11212"

	// ErrGenerateServiceCode is generated when a service cannot be generated
	// for the given resource
	ErrGenerateServiceCode = "meshkit-11213"

	// ErrConstructingRestHelperCode is generated when a rest helper cannot be generated
	// for the generated service
	ErrConstructingRestHelperCode = "meshkit-11214"

	// ErrCreatingServiceCode is generated when there is an error deploying the service
	ErrCreatingServiceCode = "meshkit-11215"

	// ErrPodHasNoLabelsCode is generated when there is an error
	ErrPodHasNoLabelsCode = "meshkit-11216"

	// ErrServiceHasNoSelectorsCode is generated when there is an error
	ErrServiceHasNoSelectorsCode = "meshkit-11217"

	// ErrInvalidDeploymentNoSelectorsLabelsCode is generated when there is an error
	ErrInvalidDeploymentNoSelectorsLabelsCode = "meshkit-11218"

	// ErrInvalidDeploymentNoSelectorsCode is generated when there is an error
	ErrInvalidDeploymentNoSelectorsCode = "meshkit-11219"

	// ErrInvalidReplicaNoSelectorsLabelsCode is generated when there is an error
	ErrInvalidReplicaNoSelectorsLabelsCode = "meshkit-11220"

	// ErrInvalidReplicaSetNoSelectorsCode is generated when there is an error
	ErrInvalidReplicaSetNoSelectorsCode = "meshkit-11221"

	// ErrNoPortsFoundForHeadlessResourceCode is generated when there is an error
	ErrNoPortsFoundForHeadlessResourceCode = "meshkit-11222"

	// ErrUnknownSessionAffinityErrCode is generated when there is an error
	ErrUnknownSessionAffinityErrCode = "meshkit-11223"

	// ErrMatchExpressionsConvertionErrCode is generated when there is an error
	ErrMatchExpressionsConvertionErrCode = "meshkit-11224"

	// ErrFailedToExtractPodSelectorErrCode is generated when there is an error
	ErrFailedToExtractPodSelectorErrCode = "meshkit-11225"

	// ErrFailedToExtractPortsCode is generated when there is an error
	ErrFailedToExtractPortsCode = "meshkit-11226"

	// ErrFailedToExtractProtocolsErrCode is generated when there is an error
	ErrFailedToExtractProtocolsErrCode = "meshkit-11227"

	// ErrCannotExposeObjectErrCode is generated when there is an error
	ErrCannotExposeObjectErrCode = "meshkit-11228"
)

// Custom errors and generators - These errors are not meshkit errors, they are intended to be wrapped
// with more common and generic meshkit errors

var (
	// ErrPodHasNoLabels is the error for pods with no labels
	ErrPodHasNoLabels = errors.New(ErrPodHasNoLabelsCode, errors.Alert, 
		[]string{"The pod has no labels and cannot be exposed."}, 
		[]string{}, 
		[]string{"Pod definition is missing required labels."}, 
		[]string{"Please add appropriate labels to the pod specification."})

	// ErrServiceHasNoSelectors is the error for service with no selectors
	ErrServiceHasNoSelectors = errors.New(ErrServiceHasNoSelectorsCode, errors.Alert, 
		[]string{"The service has no pod selector set."}, 
		[]string{}, 
		[]string{"Service definition is missing required pod selectors."}, 
		[]string{"Please add appropriate pod selectors to the service specification."})

	// ErrInvalidDeploymentNoSelectorsLabels is the error for deployment (v1beta1) with no selectors and labels
	ErrInvalidDeploymentNoSelectorsLabels = errors.New(ErrInvalidDeploymentNoSelectorsLabelsCode, errors.Alert, 
		[]string{"The deployment has no labels or selectors and cannot be exposed."}, 
		[]string{}, 
		[]string{"Deployment definition is missing required labels and selectors."}, 
		[]string{"Please add appropriate labels and selectors to the deployment specification."})

	// ErrInvalidDeploymentNoSelectors is the error for deployment (v1) with no selectors
	ErrInvalidDeploymentNoSelectors = errors.New(ErrInvalidDeploymentNoSelectorsCode, errors.Alert, 
		[]string{"Invalid deployment: No selectors present, therefore cannot be exposed."}, 
		[]string{}, 
		[]string{"Deployment definition is missing required selectors."}, 
		[]string{"Please add appropriate selectors to the deployment specification."})

	// ErrInvalidReplicaNoSelectorsLabels is the error for replicaset (v1beta1) with no selectors and labels
	ErrInvalidReplicaNoSelectorsLabels = errors.New(ErrInvalidReplicaNoSelectorsLabelsCode, errors.Alert, 
		[]string{"The replica set has no labels or selectors and cannot be exposed."}, 
		[]string{}, 
		[]string{"ReplicaSet definition is missing required labels and selectors."}, 
		[]string{"Please add appropriate labels and selectors to the ReplicaSet specification."})

	// ErrInvalidReplicaSetNoSelectors is the error for replicaset (v1) with no selectors
	ErrInvalidReplicaSetNoSelectors = errors.New(ErrInvalidReplicaSetNoSelectorsCode, errors.Alert, 
		[]string{"Invalid ReplicaSet: No selectors present, therefore cannot be exposed."}, 
		[]string{}, 
		[]string{"ReplicaSet definition is missing required selectors."}, 
		[]string{"Please add appropriate selectors to the ReplicaSet specification."})

	// ErrNoPortsFoundForHeadlessResource is the error when no ports are found for non headless resource
	ErrNoPortsFoundForHeadlessResource = errors.New(ErrNoPortsFoundForHeadlessResourceCode, errors.Alert, 
		[]string{"No ports found for the non-headless resource."}, 
		[]string{}, 
		[]string{"Resource definition is missing port specifications."}, 
		[]string{"Please specify the required ports in the resource definition."})
)

// ErrUnknownSessionAffinityErr is the error for unknown session affinity
func ErrUnknownSessionAffinityErr(sa SessionAffinity) error {
	return errors.New(ErrUnknownSessionAffinityErrCode, errors.Alert, 
		[]string{"Unknown session affinity: " + string(sa) + "."}, 
		[]string{}, 
		[]string{"The specified session affinity type is not supported."}, 
		[]string{"Please use a supported session affinity type."})
}

// ErrMatchExpressionsConvertionErr is the error for failed match expression conversion
func ErrMatchExpressionsConvertionErr(me []metav1.LabelSelectorRequirement) error {
	return errors.New(ErrMatchExpressionsConvertionErrCode, errors.Alert, 
		[]string{"Could not convert expressions to map-based selector format."}, 
		[]string{}, 
		[]string{"Label selector expressions are invalid or unsupported."}, 
		[]string{"Please verify the label selector expressions are valid."})
}

// ErrFailedToExtractPodSelectorErr is the error for failed to extract pod selector
func ErrFailedToExtractPodSelectorErr(object runtime.Object) error {
	return errors.New(ErrFailedToExtractPodSelectorErrCode, errors.Alert, 
		[]string{"Cannot extract pod selector from " + object.GetObjectKind().GroupVersionKind().Kind + "."}, 
		[]string{}, 
		[]string{"Resource definition has invalid or missing pod selectors."}, 
		[]string{"Please ensure the resource has valid pod selectors defined."})
}

// ErrFailedToExtractPorts is the error for failed to extract ports
func ErrFailedToExtractPorts(object runtime.Object) error {
	return errors.New(ErrFailedToExtractPortsCode, errors.Alert, 
		[]string{"Cannot extract ports from " + object.GetObjectKind().GroupVersionKind().Kind + "."}, 
		[]string{}, 
		[]string{"Resource definition has invalid or missing port specifications."}, 
		[]string{"Please ensure the resource has valid port specifications defined."})
}

// ErrFailedToExtractProtocolsErr is the error for extracting ports
func ErrFailedToExtractProtocolsErr(object runtime.Object) error {
	return errors.New(ErrFailedToExtractProtocolsErrCode, errors.Alert, 
		[]string{"Cannot extract protocols from " + object.GetObjectKind().GroupVersionKind().Kind + "."}, 
		[]string{}, 
		[]string{"Resource definition has invalid or missing protocol specifications."}, 
		[]string{"Please ensure the resource has valid protocol specifications defined."})
}

// ErrCannotExposeObjectErr is the error if the given object cannot be exposed
func ErrCannotExposeObjectErr(kind schema.GroupKind) error {
	return errors.New(ErrCannotExposeObjectErrCode, errors.Alert, 
		[]string{"Cannot expose a " + kind.String() + "."}, 
		[]string{}, 
		[]string{"The specified resource type cannot be exposed."}, 
		[]string{"Please verify that the resource type supports being exposed."})
}

// Meshkit errors - These errors are intended to wrap more specific errors while maintaining
// a stack trace within the errors

// ErrExposeResource is the error when there is an error exposing the kubernetes resource
func ErrExposeResource(err error) error {
	return errors.New(ErrExposeResourceCode, errors.Alert, 
		[]string{"Failed to expose the Kubernetes resource."}, 
		[]string{err.Error()}, 
		[]string{"Resource configuration may be invalid or insufficient permissions."}, 
		[]string{"Please verify the resource configuration and permissions."})
}

// ErrGettingResource is the error when there is an error getting the kubernetes resource
func ErrGettingResource(err error) error {
	return errors.New(ErrGettingResourceCode, errors.Alert, 
		[]string{"Failed to get the Kubernetes resource."}, 
		[]string{err.Error()}, 
		[]string{"Resource may not exist or insufficient permissions."}, 
		[]string{"Please verify the resource exists and check permissions."})
}

// ErrTraverser is the error is collection of error generated while traversing the resources
func ErrTraverser(err error) error {
	return errors.New(ErrTraverserCode, errors.Alert, 
		[]string{"Error traversing Kubernetes resources."}, 
		[]string{err.Error()}, 
		[]string{"Resource structure may be invalid or permissions insufficient."}, 
		[]string{"Please verify resource structure and permissions."})
}

// ErrResourceCannotBeExposed is the error if the given resource cannot be exposed
func ErrResourceCannotBeExposed(err error, resourceKind string) error {
	return errors.New(ErrResourceCannotBeExposedCode, errors.Alert, 
		[]string{"Resource type " + resourceKind + " cannot be exposed."}, 
		[]string{err.Error()}, 
		[]string{"The resource type does not support being exposed."}, 
		[]string{"Please verify that the resource type can be exposed."})
}

// ErrSelectorBasedMap is the error when the given resource's selectors can't
// be parsed to a map
func ErrSelectorBasedMap(err error) error {
	return errors.New(ErrSelectorBasedMapCode, errors.Alert, 
		[]string{"Failed to parse resource selectors to map."}, 
		[]string{err.Error()}, 
		[]string{"Selector format may be invalid."}, 
		[]string{"Please verify the selector format is correct."})
}

// ErrProtocolBasedMap is the error when the given resource's protocols can't
// be parsed to a map
func ErrProtocolBasedMap(err error) error {
	return errors.New(ErrProtocolBasedMapCode, errors.Alert, 
		[]string{"Failed to parse resource protocols to map."}, 
		[]string{err.Error()}, 
		[]string{"Protocol format may be invalid."}, 
		[]string{"Please verify the protocol format is correct."})
}

// ErrLabelBasedMap is the error when the given resource's labels can't
// be parsed to a map
func ErrLabelBasedMap(err error) error {
	return errors.New(ErrLableBasedMapCode, errors.Alert, 
		[]string{"Failed to parse resource labels to map."}, 
		[]string{err.Error()}, 
		[]string{"Label format may be invalid."}, 
		[]string{"Please verify the label format is correct."})
}

// ErrPortParsing is the error when the given resource's ports can't
// be parsed to a slice
func ErrPortParsing(err error) error {
	return errors.New(ErrPortParsingCode, errors.Alert, 
		[]string{"Failed to parse resource ports to slice."}, 
		[]string{err.Error()}, 
		[]string{"Port specification format may be invalid."}, 
		[]string{"Please verify the port specification format is correct."})
}

// ErrGenerateService is the error when a service cannot be generated
// for the given resource
func ErrGenerateService(err error) error {
	return errors.New(ErrGenerateServiceCode, errors.Alert, 
		[]string{"Failed to generate service for the resource."}, 
		[]string{err.Error()}, 
		[]string{"Resource configuration may be invalid for service generation."}, 
		[]string{"Please verify the resource configuration is valid for service generation."})
}

// ErrConstructingRestHelper is the error when a rest helper cannot be generated
// for the generated service
func ErrConstructingRestHelper(err error) error {
	return errors.New(ErrConstructingRestHelperCode, errors.Alert, 
		[]string{"Failed to construct REST helper for the service."}, 
		[]string{err.Error()}, 
		[]string{"Service configuration may be invalid for REST helper construction."}, 
		[]string{"Please verify the service configuration is valid."})
}

// ErrCreatingService is the error when there is an error deploying the service
func ErrCreatingService(err error) error {
	return errors.New(ErrCreatingServiceCode, errors.Alert, 
		[]string{"Failed to create the service."}, 
		[]string{err.Error()}, 
		[]string{"Service configuration may be invalid or insufficient permissions."}, 
		[]string{"Please verify the service configuration and permissions."})
}
