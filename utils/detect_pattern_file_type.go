package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"cuelang.org/go/cue/errors"
	"gopkg.in/yaml.v3"
)

// Function to identify the type of input
func IdentifyInputType(data []byte) (string, error) {
	if isMesheryDesign(data) {
		return "Meshery Design", nil
	}
	if isDockerCompose(data) {
		return "Docker Compose", nil
	}
	if isHelmChart(data) {
		return "Helm Chart", nil
	}
	if isK8sManifest(data) {
		return "Kubernetes Manifest", nil
	}
	return "", errors.New("unknown type")
}

// Check if the input is a Meshery design
func isMesheryDesign(data []byte) bool {
	var tempMap map[string]interface{}

	// Try unmarshaling as JSON; if it fails, try YAML
	if err := json.Unmarshal(data, &tempMap); err != nil {
		var yamlMap map[string]interface{}
		if yaml.Unmarshal(data, &yamlMap) != nil {
			return false
		}

		// Convert YAML to JSON format
		yamlToJSON, err := json.Marshal(yamlMap)
		if err != nil {
			return false
		}

		// Unmarshal JSON back into tempMap
		if json.Unmarshal(yamlToJSON, &tempMap) != nil {
			return false
		}
	}

	// Check for schemaVersion key
	schemaVersion, exists := tempMap["schemaVersion"].(string)
	if !exists {
		return false
	}

	// Validate schemaVersion for Meshery Design
	if strings.HasPrefix(schemaVersion, "designs.meshery.io") {
		return true
	}

	return false
}

// Check if the input is a Docker Compose file
func isDockerCompose(data []byte) bool {
	dockerComposeKeys := []string{"version", "services"}
	content := string(data)
	for _, key := range dockerComposeKeys {
		if !strings.Contains(content, key+":") {
			return false
		}
	}
	return true
}

// Check if the input is a Helm chart (.tgz file)
func isHelmChart(data []byte) bool {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return false
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		if strings.HasSuffix(header.Name, "Chart.yaml") {
			return true
		}
	}
	return false
}

// Check if the input is a Kubernetes manifest
func isK8sManifest(data []byte) bool {
	k8sRegex := regexp.MustCompile(`(?m)^kind:\s+\w+`)
	return k8sRegex.Match(data)
}
