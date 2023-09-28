package patterns

import (
	"github.com/layer5io/meshkit/errors"
)

// Please reference the following before contributing an error code:
// https://docs.meshery.io/project/contributing/contributing-error
// https://github.com/meshery/meshkit/blob/master/errors/errors.go
const (
	ErrGetK8sComponentsCode     = "11100"
	ErrParseK8sManifestCode     = "11101"
	ErrCreatePatternServiceCode = "11102"
	ErrPatternFromJSONCode      = "11103"
)

func ErrGetK8sComponents(err error) error {
	return errors.New(ErrGetK8sComponentsCode, errors.Alert, []string{"Kubernetes components unavailable for model registration"}, []string{err.Error()}, []string{"Invalid kubeconfig", "Filters passed incorrectly in kubeconfig", "Attempt to retrieve API resources from Kubernetes server failed."}, []string{"Ensure configuration filters provided are in alignment with output from /openapi/v2"})
}

func ErrParseK8sManifest(err error) error {
	return errors.New(ErrParseK8sManifestCode, errors.Alert, []string{"Failed to parse Kubernetes Manifest and translate into JSON"}, []string{err.Error()}, []string{"Ensure Kubernetes Manifests are valid. Use restricted YAML features like using only \"strings\" in field names."}, []string{"Ensure YAML is a valid Kubernetes Manifest"})
}

func ErrCreatePatternService(err error) error {
	return errors.New(ErrCreatePatternServiceCode, errors.Alert, []string{"Failed to create design service from Kubernetes Manifest"}, []string{err.Error()}, []string{"Invalid Kubernetes Manifest", "Could not identify the resource(s) contained in the supplied Kubernetes Manifest"}, []string{"Verify that Meshery Adapters are running.", "Verify that Meshery has successfully identified and registered Kubernetes components."})
}

func ErrPatternFromJSON(err error) error {
	return errors.New(ErrPatternFromJSONCode, errors.Alert, []string{"Could not create Meshery Design file"}, []string{err.Error()}, []string{"Invalid or corrupt data inside of Design body.", "Design service name is empty for one or more services.", "The format of information in the _data is invalid."}, []string{"Ensure contents of the design are not malformed.", "Verify that all service names are present.", "Ensure that the _data field has \"settings\" field."})
}
