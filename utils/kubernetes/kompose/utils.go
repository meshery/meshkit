package kompose

import (
	"github.com/layer5io/meshkit/utils/kubernetes/kompose/models"
	"gopkg.in/yaml.v2"
)

// IsManifestADockerCompose takes in a manifest and returns true only when the given manifest is a 'valid' docker compose file
func IsManifestADockerCompose(yamlManifest []byte) bool {
	data := models.DockerComposeFile{}
	if err := yaml.Unmarshal(yamlManifest, &data); err != nil {
		return false
	}
	if data.Version == "" {
		return false
	}
	return true
}
