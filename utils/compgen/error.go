package compgen

import "github.com/layer5io/meshkit/errors"

const (
	ErrCrdYamlCode = "11088"
)

func ErrCrdYaml(err error) error {
	return errors.New(ErrCrdYamlCode, errors.Alert, []string{"Given CRD is not a valid yaml"}, []string{err.Error()}, []string{""}, []string{"Make sure that the crds provided are all valid YAML"})
}
