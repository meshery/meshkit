package registry

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrUnknownHostCode                 = "meshkit-11146"
	ErrEmptySchemaCode                 = "meshkit-11098"
	ErrMarshalingRegisteryAttemptsCode = "meshkit-11099"
	ErrWritingRegisteryAttemptsCode    = "meshkit-11100"
	ErrRegisteringEntityCode           = "meshkit-11101"
	ErrUnknownHostInMapCode            = "meshkit-11102"
	ErrCreatingUserDataDirectoryCode   = "meshkit-11103"
)

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, []string{"host is not supported"}, []string{err.Error()}, []string{"The component's host is not supported by the version of server you are running"}, []string{"Try upgrading to latest available version"})
}
func ErrUnknownHostInMap() error {
	return errors.New(
		ErrUnknownHostInMapCode, errors.Alert, []string{"Host not found in registry logs."}, nil, []string{"The specified host does not have any associated registry logs or is unrecognized.", "Ensure the host name is correct and exists in the registry logs.", "Refer to /server/cmd/registryLogs.txt for more details."}, []string{"Verify the host name used during the registration process.", "Check the registry logs file for potential errors and additional information."})
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
	return errors.New(ErrRegisteringEntityCode, errors.Alert, []string{fmt.Sprintf("The import process for a registrant %s encountered difficulties,due to which %s. Specific issues during the import process resulted in certain entities not being successfully registered in the table.", hostName, failedMsg)}, []string{fmt.Sprintf("For registrant %s %s", hostName, failedMsg)}, []string{"Could be because of empty schema or some issue with the json or yaml file"}, []string{"Check /server/cmd/registery_attempts.json for futher details"})
}
func ErrCreatingUserDataDirectory(dir string) error {
	return errors.New(ErrCreatingUserDataDirectoryCode, errors.Fatal, []string{"Unable to create the directory for storing user data at: ", dir}, []string{"Unable to create the directory for storing user data at: ", dir}, []string{}, []string{})
}
