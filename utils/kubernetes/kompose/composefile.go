package kompose

import (
	"bytes"
	"fmt"
	"io"

	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
	"cuelang.org/go/encoding/yaml"
	yamlv3 "gopkg.in/yaml.v3"
)

type DockerComposeFile []byte

func (dc *DockerComposeFile) Validate(schema []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrValidateDockerComposeFile(fmt.Errorf("panic: %v", r))
		}
	}()

	// Check if the YAML contains multiple documents
	// Docker Compose files are single-document YAML files by specification
	if hasMultipleDocuments(*dc) {
		return ErrMultipleDocuments()
	}

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

// hasMultipleDocuments checks if the YAML data contains multiple documents
// by using the yaml.v3 decoder to detect document separators
func hasMultipleDocuments(data []byte) bool {
	decoder := yamlv3.NewDecoder(bytes.NewReader(data))

	// Try to decode the first document
	var first interface{}
	if err := decoder.Decode(&first); err != nil {
		// If we can't decode the first document, treat as single (will fail validation anyway)
		return false
	}

	// Try to decode a second document
	var second interface{}
	err := decoder.Decode(&second)

	// If err is EOF, there's only one document
	// If err is nil, there are multiple documents
	return err != io.EOF
}
