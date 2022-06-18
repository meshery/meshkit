package kompose

import "github.com/layer5io/meshkit/errors"

const (
	ErrCvrtKomposeCode         = "11075"
	ErrNoVersionCode           = "11077"
	ErrIncompatibleVersionCode = "11078"
)

func ErrCvrtKompose(err error) error {
	return errors.New(ErrCvrtKomposeCode, errors.Alert, []string{"Error converting the docker compose file into kubernetes manifests"}, []string{err.Error()}, []string{"Could not convert docker-compose file into kubernetes manifests"}, []string{"Make sure the docker-compose file is valid", ""})
}

func ErrNoVersion() error {
	return errors.New(ErrNoVersionCode, errors.Alert, []string{"version not found in the docker compose file"}, []string{"The docker compose file does not have version field in it. The underlying tool that is used for conversion mandates the presence of version field."}, []string{"Since the Docker Compose specification does not mandate the version field from version 3 onwards, most sources do not provide them."}, []string{"Make sure that the compose file has version specified,", "Add any version less than or equal to 3.3 if you cannot get the exact version from the source"})
}

func ErrIncompatibleVersion() error {
	return errors.New(ErrIncompatibleVersionCode, errors.Alert, []string{"This version of docker compose file is not compatible."}, []string{"This docker compose file is invalid since it's version is incompatible."}, []string{"docker compose file with version greater than 3.3 is probably being used"}, []string{"Make sure that the compose file has version less than or equal to 3.3,", ""})
}
