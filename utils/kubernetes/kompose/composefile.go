package kompose

import (
	"bytes"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/encoding/jsonschema"
	"cuelang.org/go/encoding/yaml"
)

type DockerComposeFile []byte

func (dc *DockerComposeFile) Validate(schema []byte) error {
	// Extract only the first document from the YAML if it's a multi-document YAML.
	// Docker Compose files are single-document YAML files, so we only validate
	// the first document. This prevents validation errors when the input is a
	// multi-resource Kubernetes manifest.
	firstDoc := extractFirstYAMLDocument(*dc)

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
	err = yaml.Validate(firstDoc, sv)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	return nil
}

// extractFirstYAMLDocument extracts the first document from a potentially
// multi-document YAML stream. If the input contains only one document,
// it returns the input unchanged.
func extractFirstYAMLDocument(data []byte) []byte {
	// Look for the document separator "---" in the YAML
	separator := []byte("\n---\n")
	if idx := bytes.Index(data, separator); idx != -1 {
		// Return only the first document
		return data[:idx]
	}

	// Also check for "---" at the start of subsequent lines (with possible whitespace)
	// This handles cases where the separator might have different spacing
	separatorAlt := []byte("\n---")
	if idx := bytes.Index(data, separatorAlt); idx != -1 {
		// Check if this is followed by whitespace or newline (proper YAML separator)
		afterSep := idx + len(separatorAlt)
		if afterSep < len(data) {
			nextChar := data[afterSep]
			if nextChar == '\n' || nextChar == '\r' || nextChar == ' ' || nextChar == '\t' {
				return data[:idx]
			}
		}
	}

	// No separator found, return the entire data
	return data
}
