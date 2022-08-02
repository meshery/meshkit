package kompose

import "github.com/layer5io/meshkit/errors"

const (
	ErrCvrtKomposeCode               = "11075"
	ErrValidateDockerComposeFileCode = "11084"
	ErrIncompatibleVersionCode       = "11083"
	ErrNoVersionCode                 = "11077"
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
