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

	// Defer cleanup to ensure temp files are removed even on error.
	defer func() {
		// We log cleanup errors but do not return them, as the primary function error is more important.
		if err := os.Remove(tempFilePath); err != nil && !os.IsNotExist(err) {
			// A logger should be used here to warn about the failed cleanup.
			// e.g., log.Warnf("failed to remove temporary file %s: %v", tempFilePath, err)
		}
		if err := os.Remove(resultFilePath); err != nil && !os.IsNotExist(err) {
			// e.g., log.Warnf("failed to remove result file %s: %v", resultFilePath, err)
		}
	}()

	// Create the temp file
	if err := utils.CreateFile(dockerCompose, "temp.data", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Format Docker Compose file for Kompose
	err = formatComposeFile(&dockerCompose)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Check version compatibility
	err = versionCheck(dockerCompose)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Initialize Kompose client
	komposeClient, err := client.NewClient()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Set up Convert options
	ConvertOpt := client.ConvertOptions{
		InputFiles:              []string{tempFilePath},
		OutFile:                 resultFilePath,
		GenerateNetworkPolicies: true,
	}

	// Convert using Kompose client
	_, err = komposeClient.Convert(ConvertOpt)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Read the result file
	result, err := os.ReadFile(resultFilePath)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	return string(result), nil
}

// cleanup removes temporary files
func cleanup(tempFilePath, resultFilePath string) error {
	// Try to remove tempFilePath
	if err := os.Remove(tempFilePath); err != nil {
		return fmt.Errorf("failed to remove temp file %s: %w", tempFilePath, err)
	}

	// Try to remove resultFilePath
	if err := os.Remove(resultFilePath); err != nil {
		return fmt.Errorf("failed to remove result file %s: %w", resultFilePath, err)
	}

	return nil // No errors
}

// formatComposeFile takes in a pointer to the compose file byte array and formats it
// so that it is compatible with Kompose. It expects a validated Docker Compose file.
func formatComposeFile(yamlManifest *DockerComposeFile) error {
	data := composeFile{}
	err := yaml.Unmarshal(*yamlManifest, &data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal compose file: %w", err)
	}

	// Marshal it again to ensure it is in the correct format for Kompose
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal compose file: %w", err)
	}

	*yamlManifest = out
	return nil
}

// versionCheck checks if the version in the Docker Compose file is compatible with Kompose.
// It expects a valid Docker Compose YAML and returns an error if the version is not supported.
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

// composeFile represents the structure of the Docker Compose file version.
type composeFile struct {
	Version string `yaml:"version,omitempty"`
}
