package utils

import "github.com/layer5io/meshkit/errors"

func ErrUnmarshal(key string, err error) error {
	return errors.NewDefault(errors.ErrUnmarshal, "Unmarshal error for key: "+key+", error: "+err.Error())
}

func ErrMarshal(err error) error {
	return errors.NewDefault(errors.ErrMarshal, "Marshal error, Description: "+err.Error())
}

func ErrGetBool(key string, err error) error {
	return errors.NewDefault(errors.ErrGetBool, "Error while getting Boolean value for key: "+key+", error: "+err.Error())
}
