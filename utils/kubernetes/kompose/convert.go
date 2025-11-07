package kompose

import (
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

	// Format the compose file before creating the temporary file
	formatComposeFile(&dockerCompose)
	err = versionCheck(dockerCompose)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Extract env_file references and create empty files to prevent errors
	envFiles := extractEnvFileReferences(dockerCompose)
	createdEnvFiles, err := createEmptyEnvFiles(mesheryDir, envFiles)
	if err != nil {
		return "", ErrCvrtKompose(err)
	}

	// Create the temporary file with the formatted compose content
	if err := utils.CreateFile(dockerCompose, "temp.data", mesheryDir); err != nil {
		return "", ErrCvrtKompose(err)
	}

	defer func() {
		os.Remove(tempFilePath)
		os.Remove(resultFilePath)
		// Clean up created env files
		for _, envFile := range createdEnvFiles {
			os.Remove(envFile)
		}
	}()

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
	Version  string                            `yaml:"version,omitempty"`
	Services map[string]map[string]interface{} `yaml:"services,omitempty"`
	Networks map[string]interface{}            `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}            `yaml:"volumes,omitempty"`
	Configs  map[string]interface{}            `yaml:"configs,omitempty"`
	Secrets  map[string]interface{}            `yaml:"secrets,omitempty"`
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
// This function ensures version is treated as a string (so "3.3" and 3.3 are handled correctly)
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

// extractEnvFileReferences parses the compose file and extracts all env_file references
func extractEnvFileReferences(yamlManifest DockerComposeFile) []string {
	data := composeFile{}
	err := yaml.Unmarshal(yamlManifest, &data)
	if err != nil {
		return []string{}
	}
	
	envFiles := []string{}
	if data.Services != nil {
		for _, service := range data.Services {
			if service != nil {
				if envFileValue, exists := service["env_file"]; exists {
					// env_file can be a string or an array of strings
					switch v := envFileValue.(type) {
					case string:
						envFiles = append(envFiles, v)
					case []interface{}:
						for _, item := range v {
							if str, ok := item.(string); ok {
								envFiles = append(envFiles, str)
							}
						}
					}
				}
			}
		}
	}
	
	return envFiles
}

// createEmptyEnvFiles creates empty env files at the specified locations relative to mesheryDir
// Returns the list of created file paths for cleanup
func createEmptyEnvFiles(mesheryDir string, envFiles []string) ([]string, error) {
	createdFiles := []string{}
	
	for _, envFile := range envFiles {
		// Resolve the path relative to mesheryDir
		fullPath := filepath.Join(mesheryDir, envFile)
		
		// Create parent directories if they don't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return createdFiles, err
		}
		
		// Check if file already exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// Create empty file
			file, err := os.Create(fullPath)
			if err != nil {
				return createdFiles, err
			}
			file.Close()
			createdFiles = append(createdFiles, fullPath)
		}
	}
	
	return createdFiles, nil
}
