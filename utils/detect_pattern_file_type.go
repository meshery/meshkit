package utils

import (
	"bytes"
	"errors"
	"strings"

	composeLoader "github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/schema"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
)

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

func isMesheryDesign(data []byte) bool {
	var mesheryPattern struct {
		SchemaVersion string        `yaml:"schemaVersion"`
		Name          string        `yaml:"name"`
		Components    []interface{} `yaml:"components"`
	}

	err := yaml.Unmarshal(data, &mesheryPattern)
	return err == nil &&
		strings.HasPrefix(mesheryPattern.SchemaVersion, "designs.meshery.io") &&
		mesheryPattern.Name != "" &&
		len(mesheryPattern.Components) > 0
}

func isDockerCompose(yamlData []byte) bool {
	dict, err := composeLoader.ParseYAML(yamlData)
	if err != nil {
		return false
	}
	if err := schema.Validate(dict); err != nil {
		return false
	}
	if _, ok := dict["services"]; !ok {
		return false
	}
	return true
}

func isHelmChart(data []byte) bool {
	chart, err := loader.LoadArchive(bytes.NewReader(data))
	return err == nil && chart.Metadata != nil && chart.Metadata.Name != ""
}

func isK8sManifest(data []byte) bool {
	var manifest struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	err := yaml.Unmarshal(data, &manifest)
	return err == nil && manifest.Kind != "" && manifest.APIVersion != ""
}
