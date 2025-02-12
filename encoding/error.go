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
	return errors.New(ErrDecodeYamlCode, errors.Alert, 
		[]string{"Error occurred while decoding YAML."}, 
		[]string{err.Error()}, 
		[]string{"Invalid YAML format or structure."}, 
		[]string{"Please verify the YAML syntax and structure is correct."})
}

func ErrUnmarshal(err error) error {
	return errors.New(ErrUnmarshalCode, errors.Alert, 
		[]string{"Unknown error occurred during unmarshaling."}, 
		[]string{err.Error()}, 
		[]string{"The data format may be invalid or corrupted."}, 
		[]string{"Please verify the data format matches the expected schema."})
}

func ErrUnmarshalInvalid(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalInvalidCode, errors.Alert, 
		[]string{"Invalid unmarshal error for type: " + typ.String() + "."}, 
		[]string{err.Error()}, 
		[]string{"The data structure does not match the expected type."}, 
		[]string{"Please ensure the data structure matches the " + typ.String() + " type definition."})
}

func ErrUnmarshalSyntax(err error, offset int64) error {
	return errors.New(ErrUnmarshalSyntaxCode, errors.Alert, 
		[]string{"Syntax error detected during unmarshal at offset: " + strconv.Itoa(int(offset)) + "."}, 
		[]string{err.Error()}, 
		[]string{"The data contains invalid syntax or formatting."}, 
		[]string{"Please check the syntax at or near position " + strconv.Itoa(int(offset)) + "."})
}

func ErrUnmarshalType(err error, value string) error {
	return errors.New(ErrUnmarshalTypeCode, errors.Alert, 
		[]string{"Type mismatch error at key: " + value + "."}, 
		[]string{err.Error()}, 
		[]string{"The data type does not match the expected type for this field."}, 
		[]string{"Please ensure the value type matches the schema definition for key: " + value + "."})
}

func ErrUnmarshalUnsupportedType(err error, typ reflect.Type) error {
	return errors.New(ErrUnmarshalUnsupportedTypeCode, errors.Alert, 
		[]string{"Unsupported type encountered: " + typ.String() + "."}, 
		[]string{err.Error()}, 
		[]string{"The data contains a type that cannot be unmarshaled."}, 
		[]string{"Please use only supported data types in your input."})
}

func ErrUnmarshalUnsupportedValue(err error, value reflect.Value) error {
	return errors.New(ErrUnmarshalUnsupportedValueCode, errors.Alert, 
		[]string{"Unsupported value encountered: " + value.String() + "."}, 
		[]string{err.Error()}, 
		[]string{"The data contains a value that cannot be unmarshaled."}, 
		[]string{"Please use only supported values in your input."})
}
