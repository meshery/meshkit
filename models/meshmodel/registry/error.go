package registry

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrUnknownHostCode                 = "meshkit-11255"
	ErrEmptySchemaCode                 = "meshkit-11256"
	ErrMarshalingRegisteryAttemptsCode = "meshkit-11257"
	ErrWritingRegisteryAttemptsCode    = "meshkit-11258"
	ErrRegisteringEntityCode           = "meshkit-11259"
	ErrUnknownHostInMapCode            = "meshkit-11260"
	ErrCreatingUserDataDirectoryCode   = "meshkit-11261"
	ErrGetByIdCode                     = "meshkit-11262"
)

func ErrGetById(err error, id string) error {
	return errors.New(
		ErrUnknownHostCode,
		errors.Alert,
		[]string{"Failed to get the entity with the given ID: " + id + "."},
		[]string{err.Error()},
		[]string{
			"Entity with the given ID may not be present in the registry.",
			"Registry might be inaccessible at the moment.",
		},
		[]string{
			"Check if your ID is correct.",
			"If the registry is inaccessible, please try again after some time.",
		},
	)
}

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, 
		[]string{"Host is not supported."}, 
		[]string{err.Error()}, 
		[]string{"The component's host is not supported by the version of server you are running."}, 
		[]string{"Try upgrading to the latest available version."})
}

func ErrUnknownHostInMap() error {
	return errors.New(
		ErrUnknownHostInMapCode, 
		errors.Alert, 
		[]string{"Host not found in registry logs."}, 
		nil, 
		[]string{
			"The specified host does not have any associated registry logs or is unrecognized.",
			"Ensure the host name is correct and exists in the registry logs.",
			"Refer to .meshery/logs/registryLogs.txt for more details.",
		}, 
		[]string{
			"Verify the host name used during the registration process.",
			"Check the registry logs file for potential errors and additional information.",
		})
}

func ErrEmptySchema() error {
	return errors.New(ErrEmptySchemaCode, errors.Alert, 
		[]string{"Empty schema for the component."}, 
		[]string{"Empty schema for the component."}, 
		[]string{"The schema is empty for the component."}, 
		[]string{"The schema for this component is empty. Please refer to the documentation or discussion forum for more details."})
}

func ErrMarshalingRegisteryAttempts(err error) error {
	return errors.New(ErrMarshalingRegisteryAttemptsCode, errors.Alert, 
		[]string{"Error marshaling RegisterAttempts to JSON."}, 
		[]string{"Error marshaling RegisterAttempts to JSON: " + err.Error()}, 
		[]string{"Failed to convert RegisterAttempts data to JSON format."}, 
		[]string{"Please check if the RegisterAttempts data is valid and properly structured."})
}

func ErrWritingRegisteryAttempts(err error) error {
	return errors.New(ErrWritingRegisteryAttemptsCode, errors.Alert, 
		[]string{"Error writing RegisteryAttempts JSON data to file."}, 
		[]string{"Error writing RegisteryAttempts JSON data to file: " + err.Error()}, 
		[]string{"Failed to write RegisteryAttempts data to the specified file."}, 
		[]string{"Please check file permissions and available disk space."})
}

func ErrRegisteringEntity(failedMsg string, hostName string) error {
	return errors.New(ErrRegisteringEntityCode, errors.Alert, 
		[]string{fmt.Sprintf("One or more entities failed to register. The import process for registrant, %s, encountered the following issue: %s.", hostName, failedMsg)}, 
		[]string{fmt.Sprintf("Registrant %s encountered %s.", hostName, failedMsg)}, 
		[]string{"Entity might be missing a required schema or have invalid JSON/YAML."}, 
		[]string{"Check 'server/cmd/registery_attempts.json' for further details."})
}

func ErrCreatingUserDataDirectory(dir string) error {
	return errors.New(ErrCreatingUserDataDirectoryCode, errors.Fatal, 
		[]string{"Unable to create the directory for storing user data at: " + dir + "."}, 
		[]string{"Unable to create the directory for storing user data at: " + dir + "."}, 
		[]string{"Insufficient permissions or disk space to create the directory."}, 
		[]string{"Please ensure you have proper permissions and sufficient disk space available."})
}
