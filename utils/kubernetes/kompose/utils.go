package kompose

import (
	"fmt"
	"strconv"

	errors "github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes/kompose/models"
	"gopkg.in/yaml.v2"
)

// IsManifestADockerCompose takes in a manifest and returns true only when the given manifest is a 'valid' docker compose file
func IsManifestADockerCompose(yamlManifest []byte) bool {
	data := models.DockerComposeFile{}
	if err := yaml.Unmarshal(yamlManifest, &data); err != nil {
		return false
	}
	if data.Services == nil {
		return false
	}
	return true
}

// VaildateDockerComposeFile takes in a manifest and returns validates it
func VaildateDockerComposeFile(yamlManifest []byte) error {
	data := models.DockerComposeFile{}
	if err := yaml.Unmarshal(yamlManifest, &data); err != nil {
		return errors.ErrUnmarshal(err)
	}
	if data.Version == "" {
		return errors.ErrMissingField(ErrNoVersion(), "Version")
	}
	versionFloatVal, err := strconv.ParseFloat(data.Version, 64)
	if err != nil {
		return errors.ErrExpectedTypeMismatch(err, "float")
	} else {
		if versionFloatVal > 3.3 {
			// kompose throws a fatal error when version exceeds 3.3
			// need this till this PR gets merged https://github.com/kubernetes/kompose/pull/1440(move away from libcompose to compose-go)
			return ErrIncompatibleVersion()
		}
	}
	if data.Services == nil {
		return errors.ErrMissingField(fmt.Errorf("Services field is missing in the docker compose file"), "Services")
	}
	return nil
}

// FormatComposeFile takes in a pointer to the compose file byte array and formats it so that it is compatible with `Kompose`
// it expects a validated docker compose file and does not validate
func FormatComposeFile(yamlManifest *[]byte) error {
	data := models.DockerComposeFile{}
	err := yaml.Unmarshal(*yamlManifest, &data)
	if err != nil {
		return errors.ErrUnmarshal(err)
	}
	data.Version = fmt.Sprintf("%s", data.Version)
	out, err := yaml.Marshal(data)
	if err != nil {
		return errors.ErrMarshal(err)
	}
	*yamlManifest = out
	return nil
}
