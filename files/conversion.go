package files

import (
	"fmt"

	"github.com/meshery/meshkit/utils/helm"
	"helm.sh/helm/v3/pkg/chart"
	"sigs.k8s.io/kustomize/api/resmap"
)

func ConvertHelmChartToKubernetesManifest(file IdentifiedFile) (string, error) {
	chart, ok := file.ParsedFile.(*chart.Chart)
	if chart != nil && !ok {
		return "", fmt.Errorf("failed to get *chart.Chart from identified file")
	}
	// empty kubernetes version because helm should figure it out
	manifest, err := helm.DryRunHelmChart(chart, "")
	if err != nil {
		return "", err
	}
	return string(manifest), nil
}

func ConvertDockerComposeToKubernetesManifest(file IdentifiedFile) (string, error) {
	parsedCompose, ok := file.ParsedFile.(ParsedCompose)
	if !ok {
		return "", fmt.Errorf("failed to get *chart.Chart from identified file")
	}

	return parsedCompose.manifest, nil
}

func ConvertKustomizeToKubernetesManifest(file IdentifiedFile) (string, error) {
	parsedKustomize, ok := file.ParsedFile.(resmap.ResMap)

	if !ok {
		return "", fmt.Errorf("failed to get *resmap.ResMap from identified file")
	}

	yamlBytes, err := parsedKustomize.AsYaml()

	return string(yamlBytes), err
}
