package encoding

import (
	"encoding/json"

	"gopkg.in/yaml.v2"
)

// Unmarshal parses the JSON/YAML data and stores the result in the value pointed to by out
func Unmarshal(data []byte, out interface{}) error {
	err := unmarshalJSON(data, out)
	if err != nil {
		err = unmarshalYAML(data, out)
		if err != nil {
			return err
		}
	}
	return nil
}

func unmarshalYAML(data []byte, result interface{}) error {
	err := yaml.Unmarshal(data, result)
	if err != nil {
		return ErrDecodeYaml(err)
	}
	return nil
}

func unmarshalJSON(data []byte, result interface{}) error {

	err := json.Unmarshal(data, result)
	if err != nil {
		if e, ok := err.(*json.SyntaxError); ok {
			return ErrUnmarshalSyntax(err, e.Offset)
		}
		if e, ok := err.(*json.UnmarshalTypeError); ok {
			return ErrUnmarshalType(err, e.Value)
		}
		if e, ok := err.(*json.UnsupportedTypeError); ok {
			return ErrUnmarshalUnsupportedType(err, e.Type)
		}
		if e, ok := err.(*json.UnsupportedValueError); ok {
			return ErrUnmarshalUnsupportedValue(err, e.Value)
		}
		if e, ok := err.(*json.InvalidUnmarshalError); ok {
			return ErrUnmarshalInvalid(err, e.Type)
		}
		return ErrUnmarshal(err)
	}
	return nil
}
