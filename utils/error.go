package utils

import (
	"reflect"
	"strconv"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrUnmarshalCode                 = "11043"
	ErrUnmarshalInvalidCode          = "11044"
	ErrUnmarshalSyntaxCode           = "11045"
	ErrUnmarshalTypeCode             = "11046"
	ErrUnmarshalUnsupportedTypeCode  = "11047"
	ErrUnmarshalUnsupportedValueCode = "11048"
	ErrMarshalCode                   = "11049"
	ErrGetBoolCode                   = "11050"
	ErrInvalidProtocolCode           = "11051"
	ErrRemoteFileNotFoundCode        = "11052"
	ErrReadingRemoteFileCode         = "11053"
	ErrReadingLocalFileCode          = "11054"

	ErrInvalidProtocol = errors.New(ErrInvalidProtocolCode, errors.Alert, []string{"invalid protocol: only http, https and file are valid protocols"}, []string{}, []string{}, []string{})
)

func ErrUnmarshal(err error) error {
	return errors.New(ErrUnmarshalCode, errors.Alert, []string{"Unmarshal unknown error: %s"}, []string{err.Error()}, []string{}, []string{})
}

func ErrUnmarshalInvalid(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalInvalidCode, errors.Alert, []string{"Unmarshal invalid error for type: ", typ.String()}, []string{err.Error()}, []string{}, []string{})
}

func ErrUnmarshalSyntax(err error, offset int64) error {
	return errors.New(ErrUnmarshalSyntaxCode, errors.Alert, []string{"Unmarshal syntax error at offest: ", strconv.Itoa(int(offset))}, []string{err.Error()}, []string{}, []string{})
}

func ErrUnmarshalType(err error, value string) error {
	return errors.New(ErrUnmarshalTypeCode, errors.Alert, []string{"Unmarshal type error at key: %s. Error: %s", value}, []string{err.Error()}, []string{}, []string{})
}

func ErrUnmarshalUnsupportedType(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalUnsupportedTypeCode, errors.Alert, []string{"Unmarshal unsupported type error at key: ", typ.String()}, []string{err.Error()}, []string{}, []string{})
}

func ErrUnmarshalUnsupportedValue(err error, value reflect.Value) error {
	return errors.New(ErrUnmarshalUnsupportedValueCode, errors.Alert, []string{"Unmarshal unsupported value error at key: ", value.String()}, []string{err.Error()}, []string{}, []string{})
}

func ErrMarshal(err error) error {
	return errors.New(ErrMarshalCode, errors.Alert, []string{"Marshal error, Description: %s"}, []string{err.Error()}, []string{}, []string{})
}

func ErrGetBool(key string, err error) error {
	return errors.New(ErrGetBoolCode, errors.Alert, []string{"Error while getting Boolean value for key: %s, error: %s", key}, []string{err.Error()}, []string{}, []string{})
}

func ErrRemoteFileNotFound(url string) error {
	return errors.New(ErrRemoteFileNotFoundCode, errors.Alert, []string{"remote file not found at", url}, []string{}, []string{}, []string{})
}

func ErrReadingRemoteFile(err error) error {
	return errors.New(ErrReadingRemoteFileCode, errors.Alert, []string{"error reading remote file"}, []string{err.Error()}, []string{}, []string{})
}

func ErrReadingLocalFile(err error) error {
	return errors.New(ErrReadingLocalFileCode, errors.Alert, []string{"error reading local file"}, []string{err.Error()}, []string{}, []string{})
}
