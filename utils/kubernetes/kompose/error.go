package kompose

import "github.com/meshery/meshkit/errors"

const (
	ErrCvrtKomposeCode               = "meshkit-11229"
	ErrValidateDockerComposeFileCode = "meshkit-11230"
	ErrIncompatibleVersionCode       = "meshkit-11231"
	ErrNoVersionCode                 = "meshkit-11232"
	ErrMultipleDocumentsCode         = "meshkit-11326"
)

func ErrCvrtKompose(err error) error {
	return errors.New(ErrCvrtKomposeCode, errors.Alert, []string{"Error converting the docker compose file into kubernetes manifests"}, []string{err.Error()}, []string{"Could not convert docker-compose file into kubernetes manifests"}, []string{"Make sure the docker-compose file is valid", ""})
}

func ErrValidateDockerComposeFile(err error) error {
	return errors.New(ErrValidateDockerComposeFileCode, errors.Alert, []string{"Invalid docker compose file"}, []string{err.Error()}, []string{""}, []string{"Make sure that the compose file is valid,", "Make sure that the schema is valid"})
}
func ErrIncompatibleVersion() error {
	return errors.New(ErrIncompatibleVersionCode, errors.Alert, []string{"This version of docker compose file is not compatible."}, []string{"This docker compose file is invalid since it's version is incompatible."}, []string{"docker compose file with version greater than 3.3 is probably being used"}, []string{"Make sure that the compose file has version less than or equal to 3.3,", ""})
}
func ErrNoVersion() error {
	return errors.New(ErrNoVersionCode, errors.Alert, []string{"version not found in the docker compose file"}, []string{"Version field not found"}, []string{"Since the Docker Compose specification does not mandate the version field from version 3 onwards, most sources do not provide them."}, []string{"Make sure that the compose file has version specified,", "Add any version less than or equal to 3.3 if you cannot get the exact version from the source"})
}

func ErrMultipleDocuments() error {
	return errors.New(ErrMultipleDocumentsCode, errors.Alert, []string{"Multiple YAML documents found in input"}, []string{"Docker Compose files must be a single YAML document"}, []string{"Input contains multiple YAML documents separated by '---'", "This may be a Kubernetes manifest with multiple resources"}, []string{"Ensure the input is a single Docker Compose file", "If importing Kubernetes manifests, use the appropriate import method"})
}
