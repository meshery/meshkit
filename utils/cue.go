package utils

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
	"cuelang.org/go/encoding/yaml"
)

func Validate(schema cue.Value, value cue.Value) (bool, []errors.Error) {
	var errs []errors.Error
	uval := value.Unify(schema)
	err := uval.Validate()
	if err != nil {
		cueErr := errors.Errors(err)
		errs = append(errs, cueErr...)
	}
	// check for required fields
	schema.Walk(func(v cue.Value) bool {
		val := value.LookupPath(v.Path())
		if !(val.Err() == nil && val.IsConcrete()) {
			cueErr := errors.Errors(errors.New(fmt.Sprintf("%v is a required field", v.Path().String())))
			errs = append(errs, cueErr...)
		}
		return true
	}, nil)
	if len(errs) != 0 {
		return false, errs
	}
	return true, make([]errors.Error, 0)
}

func GetNonConcreteFields(val cue.Value) []string {
	res := make([]string, 0)
	val.Walk(func(v cue.Value) bool {
		if !v.IsConcrete() {
			res = append(res, v.Path().String())
		}
		return true
	}, nil)
	return res
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

func JsonSchemaToCue(value string) (cue.Value, error) {
	var out cue.Value
	jsonSchema, err := json.Extract("", []byte(value))
	if err != nil {
		return out, ErrJsonSchemaToCue(err)
	}
	cueCtx := cuecontext.New()
	cueJsonSchemaExpr := cueCtx.BuildExpr(jsonSchema)
	if err = cueJsonSchemaExpr.Err(); err != nil {
		return out, ErrJsonSchemaToCue(err)
	}
	extractedSchema, err := jsonschema.Extract(cueJsonSchemaExpr, &jsonschema.Config{
		PkgName: "jsonschemeconv",
	})
	if err != nil {
		return out, ErrJsonSchemaToCue(err)
	}
	src, err := format.Node(extractedSchema)
	if err != nil {
		return out, ErrJsonSchemaToCue(err)
	}
	out = cueCtx.CompileString(string(src))
	if out.Err() != nil {
		return out, ErrJsonSchemaToCue(out.Err())
	}
	return out, nil
}

func Lookup(rootVal cue.Value, path string) (cue.Value, error) {
	res := rootVal.LookupPath(cue.ParsePath(path))
	if res.Err() != nil {
		return res, ErrCueLookup(res.Err())
	}
	if !res.Exists() {
		return res, ErrCueLookup(fmt.Errorf("Could not find the value at the path: %s", path))
	}

	return res.Value(), nil
}