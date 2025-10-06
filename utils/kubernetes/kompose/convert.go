package kompose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kubernetes/kompose/client"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshkit/utils"
	"gopkg.in/yaml.v3"
)

const DefaultDockerComposeSchemaURL = "https://raw.githubusercontent.com/compose-spec/compose-spec/master/schema/compose-spec.json"

// IsManifestADockerCompose checks whether the given manifest is a valid docker-compose file.
// schemaURL is assigned a default url if not specified.
// The error will be 'nil' if it is a valid docker compose file.
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

// Convert converts a given docker-compose file into kubernetes manifests.
// It expects a validated docker-compose file.
func Convert(dockerCompose DockerComposeFile) (string, error) {
	// Initialize a logger for this function's scope.
	log, err := logger.New("kompose-convert", logger.Options{})
	if err != nil {
		// If the logger fails, print an error and continue without logging.
		fmt.Printf("Logger initialization failed: %v\n", err)
	}

	// Get user's home directory.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Construct path to .meshery directory.
	mesheryDir := filepath.Join(homeDir, ".meshery")
	tempFilePath := filepath.Join(mesheryDir, "temp.data")
	resultFilePath := filepath.Join(mesheryDir, "result.yaml")

	// Defer cleanup to ensure temp files are removed, even on error.
	defer func() {
		// Log cleanup errors as warnings, as the primary function error is more important.
		if err := os.Remove(tempFilePath); err != nil && !os.IsNotExist(err) {
			log.Warnf("failed to remove temporary file %s: %v", tempFilePath, err)
		}
		if err := os.Remove(resultFilePath); err != nil && !os.IsNotExist(err) {
			log.Warnf("failed to remove result file %s: %v", resultFilePath, err)
		}
	}()

	// Format the Docker Compose file content to be compatible with Kompose.
	if err := formatComposeFile(&dockerCompose); err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Create a temporary file with the formatted compose data.
	if err := utils.CreateFile(dockerCompose, "temp.data", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Check for version compatibility.
	if err := versionCheck(dockerCompose); err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Initialize the Kompose client.
	komposeClient, err := client.NewClient()
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Set up the conversion options.
	ConvertOpt := client.ConvertOptions{
		InputFiles:              []string{tempFilePath},
		OutFile:                 resultFilePath,
		GenerateNetworkPolicies: true,
	}

	// Perform the conversion using the Kompose client.
	if _, err = komposeClient.Convert(ConvertOpt); err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Read the resulting Kubernetes manifest file.
	result, err := os.ReadFile(resultFilePath)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	return string(result), nil
}

// formatComposeFile takes a pointer to the compose file byte array and formats it
// to be compatible with Kompose by unmarshalling and marshalling it.
// It expects a validated Docker Compose file.
func formatComposeFile(yamlManifest *DockerComposeFile) error {
	data := composeFile{}
	if err := yaml.Unmarshal(*yamlManifest, &data); err != nil {
		return fmt.Errorf("failed to unmarshal compose file: %w", err)
	}

	// Marshal it again to ensure it's in the correct format for Kompose.
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
	}

	if versionFloatVal > 3.9 {
		return ErrIncompatibleVersion()
	}

	return nil
}

// composeFile represents the structure needed to extract the version
// from a Docker Compose file.
type composeFile struct {
	Version string `yaml:"version,omitempty"`
}
