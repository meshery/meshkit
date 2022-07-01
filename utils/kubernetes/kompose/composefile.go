package kompose

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"cuelang.org/go/cmd/cue/cmd"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/encoding/yaml"
	"github.com/layer5io/meshkit/utils"
)

type DockerComposeFile []byte

// TODO: This should be dynamic
// once we move to using encoding/jsonschema, this will be removed
var (
	DefaultSpecDirectory = filepath.Join(utils.GetHome(), ".meshery", "cue")
	DefaultSpecPath      = filepath.Join(DefaultSpecDirectory, "compose-spec.cue")
)

func generateCueSchema(jsonSchema []byte, dir string) error {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	err = utils.CreateFile(jsonSchema, "compose-spec.json", dir)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	defer os.Remove(filepath.Join(DefaultSpecDirectory, "compose-spec.json"))
	//load and parse the schema
	// NOTE: this approach makes the process slow and is not recommended
	// TODO: use encoding/jsonschema.
	importCmd, err := cmd.New([]string{"import", "-f", "--files", filepath.Join(DefaultSpecDirectory, "compose-spec.json")})
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	// no need for logs
	importCmd.SetOut(io.Discard)
	err = importCmd.Run(context.TODO())
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	return nil
}

func (dc *DockerComposeFile) Validate(schema []byte) error {
	cueCtx := cuecontext.New()
	// check if the spec is already present
	if _, err := os.Stat(DefaultSpecPath); err != nil {
		// if not, create the spec
		err = generateCueSchema(schema, DefaultSpecDirectory)
		if err != nil {
			return ErrValidateDockerComposeFile(err)
		}
	}

	cueSchema, err := utils.ReadLocalFile(DefaultSpecPath)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	sv := cueCtx.CompileString(cueSchema)
	if sv.Err() != nil {
		return ErrValidateDockerComposeFile(sv.Err())
	}
	err = yaml.Validate(*dc, sv)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	return nil
}
