package registry

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrUnknownHostCode                 = "meshkit-11247"
	ErrEmptySchemaCode                 = "meshkit-11248"
	ErrMarshalingRegisteryAttemptsCode = "meshkit-11249"
	ErrWritingRegisteryAttemptsCode    = "meshkit-11250"
	ErrRegisteringEntityCode           = "meshkit-11251"
	ErrUnknownHostInMapCode            = "meshkit-11252"
	ErrCreatingUserDataDirectoryCode   = "meshkit-11253"
  ErrGetByIdCode                     = "meshkit-11254"
)

func ErrGetById(err error, id string) error {
	return errors.New(
        ErrUnknownHostCode,
        errors.Alert,
        []string{"Failed to get the entity with the given ID: " + id},
        []string{err.Error()},
        []string{"Entity with the given ID may not be present in the registry", "Registry might be inaccessible at the moment"},
        []string{"Check if your ID is correct" , "If the registry is inaccesible, please try again after some time"},
        )

}

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, []string{"host is not supported"}, []string{err.Error()}, []string{"The component's host is not supported by the version of server you are running"}, []string{"Try upgrading to latest available version"})
}
func ErrUnknownHostInMap() error {
	return errors.New(
		ErrUnknownHostInMapCode, errors.Alert, []string{"Host not found in registry logs."}, nil, []string{"The specified host does not have any associated registry logs or is unrecognized.", "Ensure the host name is correct and exists in the registry logs.", "Refer to .meshery/logs/registryLogs.txt for more details."}, []string{"Verify the host name used during the registration process.", "Check the registry logs file for potential errors and additional information."})
}

func ErrEmptySchema() error {
	return errors.New(ErrEmptySchemaCode, errors.Alert, []string{"Empty schema for the component"}, []string{"Empty schema for the component"}, []string{"The schema is empty for the component."}, []string{"For the particular component the schema is empty. Use the docs or discussion forum for more details  "})
}
func ErrMarshalingRegisteryAttempts(err error) error {
	return errors.New(ErrMarshalingRegisteryAttemptsCode, errors.Alert, []string{"Error marshaling RegisterAttempts to JSON"}, []string{"Error marshaling RegisterAttempts to JSON: ", err.Error()}, []string{}, []string{})
}
func ErrWritingRegisteryAttempts(err error) error {
	return errors.New(ErrWritingRegisteryAttemptsCode, errors.Alert, []string{"Error writing RegisteryAttempts JSON data to file"}, []string{"Error writing RegisteryAttempts JSON data to file:", err.Error()}, []string{}, []string{})
}
func ErrRegisteringEntity(failedMsg string, hostName string) error {
	return errors.New(ErrRegisteringEntityCode, errors.Alert, []string{fmt.Sprintf("One or more entities failed to register. The import process for registrant, %s, encountered the following issue: %s.", hostName, failedMsg)}, []string{fmt.Sprintf("Registrant %s encountered %s", hostName, failedMsg)}, []string{"Entity might be missing a required schema or have invalid json / yaml."}, []string{"Check `server/cmd/registery_attempts.json` for further details."})
}
func ErrCreatingUserDataDirectory(dir string) error {
	return errors.New(ErrCreatingUserDataDirectoryCode, errors.Fatal, []string{"Unable to create the directory for storing user data at: ", dir}, []string{"Unable to create the directory for storing user data at: ", dir}, []string{}, []string{})
}
