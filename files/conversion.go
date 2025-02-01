package files

import (
	"fmt"

	dockerTypes "github.com/docker/cli/cli/compose/types"

	"github.com/layer5io/meshkit/utils/helm"
	"github.com/layer5io/meshkit/utils/kubernetes/kompose"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
)

func ConvertHelmChartToKubernetesManifest(file IdentifiedFile) (string, error) {
	chart, ok := file.ParsedFile.(*chart.Chart)
	if chart != nil && !ok {
		return "", fmt.Errorf("Failed to get *chart.Chart from identified file")
	}
	// empty kubernetes version because helm should figure it out
	manifest, err := helm.DryRunHelmChart(chart, "")
	if err != nil {
		return "", err
	}
	return string(manifest), nil
}

func ConvertDockerComposeToKubernetesManifest(file IdentifiedFile) (string, error) {
	composeConfig, ok := file.ParsedFile.(*dockerTypes.Config)
	if composeConfig != nil && !ok {
		return "", fmt.Errorf("Failed to get *chart.Chart from identified file")
	}

	yamlBytes, err := yaml.Marshal(composeConfig)

	if err != nil {
		return "", err
	}
	return kompose.Convert(yamlBytes)
}
