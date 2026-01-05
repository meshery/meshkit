package kompose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kubernetes/kompose/client"
	"github.com/meshery/meshkit/utils"
	"gopkg.in/yaml.v3"
)

const DefaultDockerComposeSchemaURL = "https://raw.githubusercontent.com/compose-spec/compose-spec/master/schema/compose-spec.json"

// Checks whether the given manifest is a valid docker-compose file.
// schemaURL is assigned a default url if not specified
// error will be 'nil' if it is a valid docker compose file
func IsManifestADockerCompose(manifest []byte, schemaURL string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrValidateDockerComposeFile(fmt.Errorf("panic: %v", r))
		}
	}()
	
	if schemaURL == "" {
		schemaURL = DefaultDockerComposeSchemaURL
	}
	schema, err := utils.ReadRemoteFile(schemaURL)
	if err != nil {
		return err
	}
	var dockerComposeFile DockerComposeFile = manifest
	err = dockerComposeFile.Validate([]byte(schema))
	return err
}

// converts a given docker-compose file into kubernetes manifests
// expects a validated docker-compose file
func Convert(dockerCompose DockerComposeFile) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = ""
			err = ErrCvrtKompose(fmt.Errorf("panic: %v", r))
		}
	}()

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Construct path to .meshery directory
	mesheryDir := filepath.Join(homeDir, ".meshery")
	tempFilePath := filepath.Join(mesheryDir, "temp.data")
	envFilePath := filepath.Join(mesheryDir, ".env")

	// create a empty .env file to avoid fatal error from kompose while reading env file
	_ = utils.CreateFile([]byte{}, ".env", mesheryDir)
	resultFilePath := filepath.Join(mesheryDir, "result.yaml")

	// create a empty .env file to avoid fatal error from kompose while reading env file
	if err := utils.CreateFile([]byte(""), ".env", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	if err := utils.CreateFile(dockerCompose, "temp.data", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	defer func() {
		_ = os.Remove(envFilePath)
		_ = os.Remove(tempFilePath)
		_ = os.Remove(resultFilePath)
	}()

	formatComposeFile(&dockerCompose)
	err = versionCheck(dockerCompose)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	komposeClient, err := client.NewClient()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	ConvertOpt := client.ConvertOptions{
		InputFiles:              []string{tempFilePath},
		OutFile:                 resultFilePath,
		GenerateNetworkPolicies: true,
	}

	_, err = komposeClient.Convert(ConvertOpt)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	resultBytes, err := os.ReadFile(resultFilePath)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	return string(resultBytes), nil
}

type composeFile struct {
	Version string `yaml:"version,omitempty"`
}

// checks if the version is compatible with `kompose`
// expects a valid docker compose yaml
// error = nil means it is compatible
func versionCheck(dc DockerComposeFile) error {
	cf := composeFile{}
	err := yaml.Unmarshal(dc, &cf)
	if err != nil {
		return utils.ErrUnmarshal(err)
	}
	// assume compatible if version is not specified
	if cf.Version == "" {
		return nil	
	}
	versionFloatVal, err := strconv.ParseFloat(cf.Version, 64)
	if err != nil {
		return utils.ErrExpectedTypeMismatch(err, "float")
	} else {
		if versionFloatVal > 3.9 {
			return ErrIncompatibleVersion()
		}
	}
	return nil
}

// formatComposeFile takes in a pointer to the compose file byte array and formats it so that it is compatible with `Kompose`
// it expects a validated docker compose file and does not validate
func formatComposeFile(yamlManifest *DockerComposeFile) {
	data := composeFile{}
	err := yaml.Unmarshal(*yamlManifest, &data)
	if err != nil {
		return
	}
	// so that "3.3" and 3.3 are treated differently by `Kompose`
	out, err := yaml.Marshal(data)
	if err != nil {
		return
	}
	*yamlManifest = out
}
