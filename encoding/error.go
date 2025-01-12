package encoding

import (
	"reflect"
	"strconv"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrDecodeYamlCode                = "meshkit-11245"
	ErrUnmarshalCode                 = "meshkit-11246"
	ErrUnmarshalInvalidCode          = "meshkit-11247"
	ErrUnmarshalSyntaxCode           = "meshkit-11248"
	ErrUnmarshalTypeCode             = "meshkit-11249"
	ErrUnmarshalUnsupportedTypeCode  = "meshkit-11250"
	ErrUnmarshalUnsupportedValueCode = "meshkit-11251"
)

// ErrDecodeYaml is the error when the yaml unmarshal fails
func ErrDecodeYaml(err error) error {
	return errors.New(ErrDecodeYamlCode, errors.Alert, []string{"Error occurred while decoding YAML"}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid YAML object"})
}

func ErrUnmarshal(err error) error {
	return errors.New(ErrUnmarshalCode, errors.Alert, []string{"Unmarshal unknown error: "}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalInvalid(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalInvalidCode, errors.Alert, []string{"Unmarshal invalid error for type: ", typ.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalSyntax(err error, offset int64) error {
	return errors.New(ErrUnmarshalSyntaxCode, errors.Alert, []string{"Unmarshal syntax error at offest: ", strconv.Itoa(int(offset))}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalType(err error, value string) error {
	return errors.New(ErrUnmarshalTypeCode, errors.Alert, []string{"Unmarshal type error at key: %s. Error: %s", value}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalUnsupportedType(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalUnsupportedTypeCode, errors.Alert, []string{"Unmarshal unsupported type error at key: ", typ.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}

func ErrUnmarshalUnsupportedValue(err error, value reflect.Value) error {
	return errors.New(ErrUnmarshalUnsupportedValueCode, errors.Alert, []string{"Unmarshal unsupported value error at key: ", value.String()}, []string{err.Error()}, []string{"Invalid object format"}, []string{"Make sure to input a valid JSON object"})
}
