package utils

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/yaml"
)

func Validate(schema cue.Value, value cue.Value) (bool, error) {
	uval := schema.Unify(value)
	if uval.Err() != nil {
		return false, uval.Err()
	}
	return true, nil
}

func JsonToCue(value []byte) (cue.Value, error) {
	var out cue.Value
	cuectx := cuecontext.New()
	expr, err := json.Extract("", value)
	if err != nil {
		return out, ErrJsonToCue(err)
	}
	out = cuectx.BuildExpr(expr)
	if out.Err() != nil {
		return out, ErrJsonToCue(out.Err())
	}
	return out, nil
}

func YamlToCue(value string) (cue.Value, error) {
	var out cue.Value
	cuectx := cuecontext.New()
	expr, err := yaml.Extract("", value)
	if err != nil {
		return out, ErrYamlToCue(err)
	}
	out = cuectx.BuildFile(expr)
	if out.Err() != nil {
		return out, ErrYamlToCue(out.Err())
	}
	return out, nil
}
