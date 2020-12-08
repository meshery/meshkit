package expose

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
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

// ErrLableBasedMap is the error when the given resource's labels can't
// be parsed to a map
func ErrLableBasedMap(err error) error {
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
