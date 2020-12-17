package utils

import (
	"fmt"
	"reflect"

	"github.com/layer5io/meshkit/errors"
)

func ErrUnmarshal(err error) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal unknown error: %s", err.Error()))
}

func ErrUnmarshalInvalid(err error, typ reflect.Type) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal invalid error for type:%v, Error:%s", typ, err.Error()))
}

func ErrUnmarshalSyntax(err error, offset int64) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal syntax error at offest: %d. Error: %s", offset, err.Error()))
}

func ErrUnmarshalType(err error, value string) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal type error at key: %s. Error: %s", value, err.Error()))
}

func ErrUnmarshalUnsupportedType(err error, typ reflect.Type) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal unsupported type error at key: %v. Error: %s", typ, err.Error()))
}

func ErrUnmarshalUnsupportedValue(err error, value reflect.Value) error {
	return errors.NewDefault(errors.ErrUnmarshal, fmt.Sprintf("Unmarshal unsupported value error at key: %v. Error: %s", value, err.Error()))
}

func ErrMarshal(err error) error {
	return errors.NewDefault(errors.ErrMarshal, fmt.Sprintf("Marshal error, Description: %s", err.Error()))
}

func ErrGetBool(key string, err error) error {
	return errors.NewDefault(errors.ErrGetBool, fmt.Sprintf("Error while getting Boolean value for key: %s, error: %s", key, err.Error()))
}

func ErrInvalidProtocol() error {
	return errors.NewDefault(errors.ErrLoadFile, "invalid protocol: only http, https and file are valid protocols")
}

func ErrRemoteFileNotFound(url string) error {
	return errors.NewDefault(errors.ErrLoadFile, fmt.Sprintf("remote file not found at %s", url))
}

func ErrReadingRemoteFile(err error) error {
	return errors.NewDefault(errors.ErrLoadFile, fmt.Sprintf("error reading remote file: %s", err))
}

func ErrReadingLocalFile(err error) error {
	return errors.NewDefault(errors.ErrLoadFile, fmt.Sprintf("error reading local file: %s", err))
}
