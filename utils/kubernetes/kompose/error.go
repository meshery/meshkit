package kompose

import "github.com/layer5io/meshkit/errors"

const (
	ErrCvrtKomposeCode = "11075"
)

func ErrCvrtKompose(err error) error {
	return errors.New(ErrCvrtKomposeCode, errors.Alert, []string{"Error converting the docker compose file into kubernetes manifests"}, []string{err.Error()}, []string{"Could not convert docker-compose file into kubernetes manifests"}, []string{"Make sure the docker-compose file is valid", ""})
}
