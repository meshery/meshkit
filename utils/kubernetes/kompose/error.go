package kompose

import "github.com/layer5io/meshkit/errors"

const (
	ErrCvrtKomposeCode               = "meshkit-11229"
	ErrValidateDockerComposeFileCode = "meshkit-11230"
	ErrIncompatibleVersionCode       = "meshkit-11231"
	ErrNoVersionCode                 = "meshkit-11232"
)

func ErrCvrtKompose(err error) error {
	return errors.New(ErrCvrtKomposeCode, errors.Alert, 
		[]string{"Error converting the Docker Compose file into Kubernetes manifests."}, 
		[]string{err.Error()}, 
		[]string{"Could not convert Docker Compose file into Kubernetes manifests."}, 
		[]string{"Make sure the Docker Compose file is valid."})
}

func ErrValidateDockerComposeFile(err error) error {
	return errors.New(ErrValidateDockerComposeFileCode, errors.Alert, 
		[]string{"Invalid Docker Compose file."}, 
		[]string{err.Error()}, 
		[]string{"The Docker Compose file format is invalid."}, 
		[]string{
			"Make sure that the compose file is valid.",
			"Make sure that the schema is valid.",
		})
}

func ErrIncompatibleVersion() error {
	return errors.New(ErrIncompatibleVersionCode, errors.Alert, 
		[]string{"This version of Docker Compose file is not compatible."}, 
		[]string{"This Docker Compose file is invalid since its version is incompatible."}, 
		[]string{"Docker Compose file with version greater than 3.3 is being used."}, 
		[]string{"Make sure that the compose file has version less than or equal to 3.3."})
}

func ErrNoVersion() error {
	return errors.New(ErrNoVersionCode, errors.Alert, 
		[]string{"Version not found in the Docker Compose file."}, 
		[]string{"Version field not found."}, 
		[]string{"Since the Docker Compose specification does not mandate the version field from version 3 onwards, most sources do not provide them."}, 
		[]string{
			"Make sure that the compose file has version specified.",
			"Add any version less than or equal to 3.3 if you cannot get the exact version from the source.",
		})
}
