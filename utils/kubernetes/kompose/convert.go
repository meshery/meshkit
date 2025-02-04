package kompose

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/kubernetes/kompose/client"
	"github.com/layer5io/meshkit/utils"
	"gopkg.in/yaml.v3"
)

const DefaultDockerComposeSchemaURL = "https://raw.githubusercontent.com/compose-spec/compose-spec/master/schema/compose-spec.json"

// Checks whether the given manifest is a valid docker-compose file.
// schemaURL is assigned a default url if not specified
// error will be 'nil' if it is a valid docker compose file
func IsManifestADockerCompose(manifest []byte, schemaURL string) error {
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
func Convert(dockerCompose DockerComposeFile) (string, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Construct path to .meshery directory
	mesheryDir := filepath.Join(homeDir, ".meshery")
	tempFilePath := filepath.Join(mesheryDir, "temp.data")
	resultFilePath := filepath.Join(mesheryDir, "result.yaml")

	if err := utils.CreateFile(dockerCompose, "temp.data", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	defer func() {
		os.Remove(tempFilePath)
		os.Remove(resultFilePath)
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

	result, err := os.ReadFile(resultFilePath)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	return string(result), nil
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
	if cf.Version == "" {
		return ErrNoVersion()
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
