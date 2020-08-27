package utils

import "github.com/layer5io/gokit/errors"

func ErrUnmarshal(key string, err error) error {
	return errors.New("ERR.UNMARSHAL", "Unmarshal error for key: "+key+", error: "+err.Error())
}

func ErrMarshal(err error) error {
	return errors.New("ERR.MARSHAL", "Marshal error, Description: "+err.Error())
}

func ErrGetBool(key string, err error) error {
	return errors.New("ERR.GETBOOL", "Error while getting Boolean value for key: "+key+", error: "+err.Error())
}
