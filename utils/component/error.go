package component

import "github.com/layer5io/meshkit/errors"

const (
	ErrCrdGenerateCode = "11088"
	ErrDefinitionCode  = "11090"
	ErrGetSchemaCode   = "11091"
)

func ErrCrdGenerate(err error) error {
	return errors.New(ErrCrdGenerateCode, errors.Alert, []string{"Could not generate component with the given CRD"}, []string{err.Error()}, []string{""}, []string{"Make sure that the crds provided are all valid YAML"})
}

func ErrGetDefinition(err error) error {
	return errors.New(ErrDefinitionCode, errors.Alert, []string{"Could not get definition for the given CRD"}, []string{err.Error()}, []string{""}, []string{"Make sure that the given CRD is valid"})
}

func ErrGetSchema(err error) error {
	return errors.New(ErrGetSchemaCode, errors.Alert, []string{"Could not get schema for the given CRD"}, []string{err.Error()}, []string{""}, []string{"Make sure that the given CRD is valid"})
}
