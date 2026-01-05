package kompose

import (
	"fmt"
	
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
	"cuelang.org/go/encoding/yaml"
)

type DockerComposeFile []byte

func (dc *DockerComposeFile) Validate(schema []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrValidateDockerComposeFile(fmt.Errorf("panic: %v", r))
		}
	}()
	
	jsonSchema, err := json.Extract("", schema)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	cueCtx := cuecontext.New()
	cueJsonSchemaExpr := cueCtx.BuildExpr(jsonSchema)
	if err = cueJsonSchemaExpr.Err(); err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	extractedSchema, err := jsonschema.Extract(cueJsonSchemaExpr, &jsonschema.Config{
		PkgName: "composespec",
	})
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	src, err := format.Node(extractedSchema)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	sv := cueCtx.CompileString(string(src))
	if sv.Err() != nil {
		return ErrValidateDockerComposeFile(sv.Err())
	}
	err = yaml.Validate(*dc, sv)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	return nil
}
