package kompose

import (
	"context"
	"io"
	"os"

	"cuelang.org/go/cmd/cue/cmd"
	"github.com/layer5io/meshkit/utils"
)

type DockerComposeFile []byte

func (dc *DockerComposeFile) Validate(schema []byte) error {
	err := utils.CreateFile(schema, "compose-spec.json", "./")
	if err != nil {
		ErrValidateDockerComposeFile(err)
	}
	//load and parse the schema
	importCmd, err := cmd.New([]string{"import", "-f", "-l", "#ComposeSpec:", "-o", "schema.cue", "compose-spec.json"})
	if err != nil {
		ErrValidateDockerComposeFile(err)
	}
	// no need for logs
	importCmd.SetOut(io.Discard)
	defer os.Remove("composefile.yaml")
	defer os.Remove("schema.cue")
	err = importCmd.Run(context.TODO())
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	//persist composefile in FS
	err = os.WriteFile("composefile.yaml", *dc, 0644)
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	// validate using cue vet
	vetCmd, err := cmd.New([]string{"vet", "-d", "#ComposeSpec", "composefile.yaml", "schema.cue"})
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}
	vetCmd.SetOut(io.Discard)
	err = vetCmd.Run(context.TODO())
	if err != nil {
		return ErrValidateDockerComposeFile(err)
	}

	return nil
}
