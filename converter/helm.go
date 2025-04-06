package converter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/layer5io/meshkit/models/patterns"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/lint/support"
)

type HelmConverter struct{}

func (h *HelmConverter) Convert(patternFile string) (string, error) {
	pattern, err := patterns.GetPatternFormat(patternFile)
	if err != nil {
		return "", err
	}
	patterns.ProcessAnnotations(pattern)

	helmChart, err := generateHelmChart(pattern)
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	fmt.Println("Home directory: ", homeDir)
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	mesheryDir := filepath.Join(homeDir, ".meshery")
	if err := os.MkdirAll(mesheryDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .meshery directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	tmpDir := filepath.Join(mesheryDir, "tmp", "helm", timestamp)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chartPath := filepath.Join(tmpDir, sanitizeHelmName(pattern.Name))
	if err := os.MkdirAll(chartPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create chart directory: %w", err)
	}

	if err := saveChartToDirectory(helmChart, chartPath); err != nil {
		return "", fmt.Errorf("failed to save chart to directory: %w", err)
	}

	if err := lintChart(chartPath); err != nil {
		return "", fmt.Errorf("chart linting failed: %w", err)
	}

	var buf bytes.Buffer
	if err := packageChart(chartPath, &buf); err != nil {
		return "", fmt.Errorf("failed to package helm chart: %w", err)
	}

	return buf.String(), nil
}

func generateNamespaceTemplate() []byte {
	return []byte(`{{- if .Values.createNamespace }}
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.global.namespace }}
  labels:
    app.kubernetes.io/managed-by: {{ .Release.Service }}
    app.kubernetes.io/instance: {{ .Release.Name }}
    helm.sh/chart: {{ include "chart.name" . }}
{{- end }}
`)
}

func generateHelmChart(patternFile *pattern.PatternFile) (*chart.Chart, error) {
	chartName := sanitizeHelmName(patternFile.Name)

	helmChart := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:        chartName,
			Version:     "0.1.0",
			Description: fmt.Sprintf("Helm chart for Meshery design %s", patternFile.Name),
			APIVersion:  chart.APIVersionV2,
			Type:        "application",
			AppVersion:  patternFile.Version,
			Keywords:    []string{"meshery", "design", "kubernetes"},
			Home:        "https://meshery.io/",
			Sources:     []string{"https://github.com/meshery/meshery"},
			Maintainers: []*chart.Maintainer{
				{
					Name:  "Meshery Authors",
					Email: "maintainers@meshery.io",
				},
			},
		},
		Templates: []*chart.File{},
		Files:     []*chart.File{},
	}

	values, err := generateValues(patternFile)
	if err != nil {
		return nil, err
	}

	values["createNamespace"] = false

	helmChart.Values = values

	helmChart.Templates = append(helmChart.Templates, &chart.File{
		Name: "templates/namespace.yaml",
		Data: generateNamespaceTemplate(),
	})

	templatesByKind, err := generateTemplates(patternFile, helmChart)
	if err != nil {
		return nil, err
	}

	helpersContent := generateHelperTemplates()
	helmChart.Templates = append(helmChart.Templates, &chart.File{
		Name: "templates/_helpers.tpl",
		Data: []byte(helpersContent),
	})

	notesContent := generateNotes(patternFile, templatesByKind)
	helmChart.Templates = append(helmChart.Templates, &chart.File{
		Name: "templates/NOTES.txt",
		Data: []byte(notesContent),
	})

	readmeContent := generateReadme(patternFile, templatesByKind)
	helmChart.Files = append(helmChart.Files, &chart.File{
		Name: "README.md",
		Data: []byte(readmeContent),
	})

	return helmChart, nil
}

func generateValues(patternFile *pattern.PatternFile) (map[string]interface{}, error) {
	values := map[string]interface{}{
		"global": map[string]interface{}{
			"namespace": "default",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "Meshery",
			},
		},
		"resources": map[string]interface{}{},
	}

	resources := values["resources"].(map[string]interface{})

	for _, comp := range patternFile.Components {
		safeName := sanitizeHelmName(comp.DisplayName)

		resourceConfig := map[string]interface{}{
			"enabled": true,
			"kind": comp.Component.Kind,
			"apiVersion": comp.Component.Version,
		}

		if comp.Component.Kind == "Deployment" || comp.Component.Kind == "StatefulSet" {
			resourceConfig["replicas"] = extractReplicas(comp, 1)
		}

		image := extractImage(comp)
		if image != "" {
			resourceConfig["image"] = image
		}

		if ns := extractNamespace(comp); ns != "" {
			resourceConfig["namespace"] = ns
		}

		if labels := extractLabels(comp); len(labels) > 0 {
			resourceConfig["labels"] = labels
		}

		if annotations := extractAnnotations(comp); len(annotations) > 0 {
			resourceConfig["annotations"] = annotations
		}

		resources[safeName] = resourceConfig
	}

	return values, nil
}

func extractReplicas(comp *component.ComponentDefinition, defaultValue int) int {
	if comp.Configuration == nil {
		return defaultValue
	}

	if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
		if replicas, ok := spec["replicas"]; ok {
			if val, ok := replicas.(int); ok {
				return val
			}
		}
	}

	return defaultValue
}

func extractImage(comp *component.ComponentDefinition) string {
	if comp.Configuration == nil {
		return ""
	}

	if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if podSpec, ok := template["spec"].(map[string]interface{}); ok {
				if containers, ok := podSpec["containers"].([]interface{}); ok && len(containers) > 0 {
					if container, ok := containers[0].(map[string]interface{}); ok {
						if image, ok := container["image"].(string); ok {
							return image
						}
					}
				}
			}
		}
	}

	return ""
}

func extractNamespace(comp *component.ComponentDefinition) string {
	if comp.Configuration == nil {
		return ""
	}

	if metadata, ok := comp.Configuration["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "default" {
			return ns
		}
	}

	return ""
}

func extractLabels(comp *component.ComponentDefinition) map[string]interface{} {
	if comp.Configuration == nil {
		return nil
	}

	if metadata, ok := comp.Configuration["metadata"].(map[string]interface{}); ok {
		if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
			return labels
		}
	}

	return nil
}

func extractAnnotations(comp *component.ComponentDefinition) map[string]interface{} {
	if comp.Configuration == nil {
		return nil
	}

	if metadata, ok := comp.Configuration["metadata"].(map[string]interface{}); ok {
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
			return annotations
		}
	}

	return nil
}

func generateTemplates(patternFile *pattern.PatternFile, helmChart *chart.Chart) (map[string][]string, error) {
	templatesByKind := make(map[string][]string)

	componentsByKind := make(map[string][]*component.ComponentDefinition)
	for _, comp := range patternFile.Components {
		if comp.Component.Kind == "" {
			continue
		}

		kind := strings.ToLower(comp.Component.Kind)
		if _, ok := componentsByKind[kind]; !ok {
			componentsByKind[kind] = []*component.ComponentDefinition{}
		}
		componentsByKind[kind] = append(componentsByKind[kind], comp)

		if _, ok := templatesByKind[kind]; !ok {
			templatesByKind[kind] = []string{}
		}
		templatesByKind[kind] = append(templatesByKind[kind], comp.DisplayName)
	}

	for kind, components := range componentsByKind {
		templateContent := generateTemplateForKind(kind, components)

		helmChart.Templates = append(helmChart.Templates, &chart.File{
			Name: fmt.Sprintf("templates/%s.yaml", kind),
			Data: []byte(templateContent),
		})
	}

	return templatesByKind, nil
}

func generateTemplateForKind(kind string, components []*component.ComponentDefinition) string {
	var templateContent bytes.Buffer

	if strings.ToLower(kind) == "namespace" {
		return ""
	}

	for _, comp := range components {
		safeName := sanitizeHelmName(comp.DisplayName)

		templateContent.WriteString(fmt.Sprintf(`{{- if (index .Values.resources "%s").enabled }}
---
apiVersion: {{ (index .Values.resources "%s").apiVersion | default "%s" }}
kind: {{ (index .Values.resources "%s").kind | default "%s" }}
metadata:
  name: %s
  namespace: {{ (index .Values.resources "%s").namespace | default .Values.global.namespace }}
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/managed-by: {{ .Release.Service }}
    helm.sh/chart: {{ include "chart.name" . }}-{{ .Chart.Version }}
`, safeName, safeName, comp.Component.Version, safeName, comp.Component.Kind, comp.DisplayName, safeName, comp.DisplayName))

		templateContent.WriteString(fmt.Sprintf(`    {{- if (index .Values.resources "%s").labels }}
    {{- toYaml (index .Values.resources "%s").labels | nindent 4 }}
    {{- end }}
`, safeName, safeName))

		templateContent.WriteString(fmt.Sprintf(`  {{- if (index .Values.resources "%s").annotations }}
  annotations:
    {{- toYaml (index .Values.resources "%s").annotations | nindent 4 }}
  {{- end }}
`, safeName, safeName))

		switch strings.ToLower(kind) {
		case "deployment", "statefulset":
			templateContent.WriteString(fmt.Sprintf(`spec:
  replicas: {{ (index .Values.resources "%s").replicas | default 1 }}
`, safeName))

			if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
				if selector, ok := spec["selector"].(map[string]interface{}); ok {
					selectorYaml, err := yaml.Marshal(selector)
					if err == nil {
						templateContent.WriteString(fmt.Sprintf(`  selector:
%s`, indentYaml(string(selectorYaml), 4)))
					}
				}
			}

			if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
				if template, ok := spec["template"].(map[string]interface{}); ok {

					templateContent.WriteString(`  template:
    metadata:
      labels:
        app: {{ include "chart.name" . }}
        {{- if (index .Values.resources "`)
					templateContent.WriteString(fmt.Sprintf("%s", safeName))
					templateContent.WriteString(`").labels }}
        {{- toYaml (index .Values.resources "`)
					templateContent.WriteString(fmt.Sprintf("%s", safeName))
					templateContent.WriteString(`").labels | nindent 8 }}
        {{- end }}
`)

					if podSpec, ok := template["spec"].(map[string]interface{}); ok {
						podSpecCopy := make(map[string]interface{})
						for k, v := range podSpec {
							if k == "containers" {
								if containers, ok := v.([]interface{}); ok && len(containers) > 0 {

									podSpecCopy["containers"] = []interface{}{
										map[string]interface{}{
											"name":  comp.DisplayName,
											"image": fmt.Sprintf(`{{ (index .Values.resources "%s").image | default "nginx:latest" }}`, safeName),
										},
									}

									if container, ok := containers[0].(map[string]interface{}); ok {
										containerCopy := podSpecCopy["containers"].([]interface{})[0].(map[string]interface{})
										for k, v := range container {
											if k != "image" && k != "name" {
												containerCopy[k] = v
											}
										}
									}
								}
							} else {
								podSpecCopy[k] = v
							}
						}

						podSpecYaml, err := yaml.Marshal(podSpecCopy)
						if err == nil {
							templateContent.WriteString(fmt.Sprintf(`    spec:
%s`, indentYaml(string(podSpecYaml), 6)))
						}
					}
				}
			}

		case "service":
			if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
				specYaml, err := yaml.Marshal(spec)
				if err == nil {
					templateContent.WriteString(fmt.Sprintf(`spec:
%s`, indentYaml(string(specYaml), 2)))
				}
			}

		case "networkpolicy":

			if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {

				specCopy := make(map[string]interface{})
				for k, v := range spec {
					specCopy[k] = v
				}

				if _, hasPodSelector := specCopy["podSelector"]; !hasPodSelector {
					if selector, hasSelector := specCopy["selector"]; hasSelector {

						specCopy["podSelector"] = selector
						delete(specCopy, "selector")
					} else {

						specCopy["podSelector"] = map[string]interface{}{}
					}
				}

				specYaml, err := yaml.Marshal(specCopy)
				if err == nil {
					templateContent.WriteString(fmt.Sprintf(`spec:
%s`, indentYaml(string(specYaml), 2)))
				}
			} else {

				templateContent.WriteString(`spec:
  podSelector: {}
`)
			}

		default:

			for key, value := range comp.Configuration {
				if key != "apiVersion" && key != "kind" && key != "metadata" {
					valueYaml, err := yaml.Marshal(value)
					if err == nil {
						templateContent.WriteString(fmt.Sprintf(`%s:
%s`, key, indentYaml(string(valueYaml), 2)))
					}
				}
			}
		}

		templateContent.WriteString(fmt.Sprintf(`
{{- end }}
`))
	}

	return templateContent.String()
}

func indentYaml(yamlStr string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(yamlStr, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}

	return strings.Join(lines, "\n")
}

func generateHelperTemplates() string {
	return `{{/* Expand the name of the chart. */}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Create chart name and version as used by the chart label. */}}
{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/* Create common labels */}}
{{- define "helpers.labels" -}}
helm.sh/chart: {{ include "chart.name" . }}-{{ .Chart.Version }}
{{ include "helpers.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/* Selector labels */}}
{{- define "helpers.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
`
}

func generateNotes(patternFile *pattern.PatternFile, templatesByKind map[string][]string) string {
	var notes bytes.Buffer

	notes.WriteString(fmt.Sprintf("Thank you for installing the \"%s\" Helm chart!\n\n", patternFile.Name))
	notes.WriteString("This chart was generated from a Meshery design.\n\n")
	notes.WriteString("The following resources have been deployed:\n")

	if services, ok := templatesByKind["service"]; ok && len(services) > 0 {
		notes.WriteString("\nServices:\n")
		for _, svc := range services {
			notes.WriteString(fmt.Sprintf("  - %s\n", svc))
		}

		notes.WriteString("\nTo access the services:\n")
		for _, svc := range services {
			notes.WriteString(fmt.Sprintf(`
  export SERVICE_PORT=$(kubectl get -o jsonpath="{.spec.ports[0].port}" services %s -n {{ .Release.Namespace }})
  export NODE_IP=$(kubectl get nodes -o jsonpath="{.items[0].status.addresses[0].address}")
  echo "Access %s at http://$NODE_IP:$SERVICE_PORT"
`, svc, svc))
		}
	}

	if deployments, ok := templatesByKind["deployment"]; ok && len(deployments) > 0 {
		notes.WriteString("\nDeployments:\n")
		for _, deploy := range deployments {
			notes.WriteString(fmt.Sprintf("  - %s\n", deploy))
		}

		notes.WriteString("\nTo check deployment status:\n")
		for _, deploy := range deployments {
			notes.WriteString(fmt.Sprintf("  kubectl get deployment %s -n {{ .Release.Namespace }}\n", deploy))
		}
	}

	if statefulsets, ok := templatesByKind["statefulset"]; ok && len(statefulsets) > 0 {
		notes.WriteString("\nStatefulSets:\n")
		for _, sts := range statefulsets {
			notes.WriteString(fmt.Sprintf("  - %s\n", sts))
		}
	}

	notes.WriteString("\nGeneral commands:\n")
	notes.WriteString("  kubectl get all -n {{ .Release.Namespace }} -l app.kubernetes.io/instance={{ .Release.Name }}\n")

	return notes.String()
}

func generateReadme(patternFile *pattern.PatternFile, templatesByKind map[string][]string) string {
	var readme bytes.Buffer

	readme.WriteString(fmt.Sprintf("# %s\n\n", patternFile.Name))
	readme.WriteString(fmt.Sprintf("Helm chart for Meshery design: %s\n\n", patternFile.Name))
	readme.WriteString(fmt.Sprintf("Version: %s\n\n", patternFile.Version))

	readme.WriteString("## Installation\n\n")
	readme.WriteString("```bash\n")
	readme.WriteString(fmt.Sprintf("# Install using default namespace\n"))
	readme.WriteString(fmt.Sprintf("helm install my-release ./%s\n\n", sanitizeHelmName(patternFile.Name)))
	readme.WriteString(fmt.Sprintf("# Install with custom namespace\n"))
	readme.WriteString(fmt.Sprintf("helm install my-release ./%s --set global.namespace=my-namespace --set createNamespace=true\n", sanitizeHelmName(patternFile.Name)))
	readme.WriteString("```\n\n")

	readme.WriteString("## Configuration\n\n")
	readme.WriteString("The following table lists the configurable parameters of the chart and their default values.\n\n")
	readme.WriteString("| Parameter | Description | Default |\n")
	readme.WriteString("| --- | --- | --- |\n")
	readme.WriteString("| `global.namespace` | Default namespace for all resources | `default` |\n")
	readme.WriteString("| `createNamespace` | Create the namespace if it doesn't exist | `false` |\n")

	for kind, resources := range templatesByKind {
		if kind == "namespace" {
			continue
		}

		for _, resourceName := range resources {
			safeName := sanitizeHelmName(resourceName)
			readme.WriteString(fmt.Sprintf("| `resources.%s.enabled` | Enable %s %s | `true` |\n",
				safeName, resourceName, kind))

			if kind == "deployment" || kind == "statefulset" {
				readme.WriteString(fmt.Sprintf("| `resources.%s.replicas` | Number of replicas for %s | `1` |\n",
					safeName, resourceName))
				readme.WriteString(fmt.Sprintf("| `resources.%s.image` | Image for %s | `As defined in design` |\n",
					safeName, resourceName))
			}
		}
	}

	readme.WriteString("\n## Resources\n\n")
	for kind, resources := range templatesByKind {
		if kind == "namespace" {
			continue
		}

		titleKind := strings.Title(kind)
		readme.WriteString(fmt.Sprintf("### %s\n\n", titleKind))
		for _, name := range resources {
			readme.WriteString(fmt.Sprintf("- %s\n", name))
		}
		readme.WriteString("\n")
	}

	return readme.String()
}

func sanitizeHelmName(name string) string {
	result := strings.ToLower(name)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	result = reg.ReplaceAllString(result, "-")

	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	result = strings.Trim(result, "-")

	return result
}

func sanitizeFileName(name string) string {
	result := strings.ReplaceAll(name, "/", "-")
	result = strings.ReplaceAll(result, "\\", "-")
	result = strings.ReplaceAll(result, ":", "-")
	result = strings.ReplaceAll(result, "*", "-")
	result = strings.ReplaceAll(result, "?", "-")
	result = strings.ReplaceAll(result, "\"", "-")
	result = strings.ReplaceAll(result, "<", "-")
	result = strings.ReplaceAll(result, ">", "-")
	result = strings.ReplaceAll(result, "|", "-")

	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	result = strings.Trim(result, "-")

	return result
}

func saveChartToDirectory(c *chart.Chart, dir string) error {

	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return err
	}

	chartsDir := filepath.Join(dir, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return err
	}

	chartYaml, err := yaml.Marshal(c.Metadata)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), chartYaml, 0644); err != nil {
		return err
	}

	valuesYaml, err := yaml.Marshal(c.Values)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), valuesYaml, 0644); err != nil {
		return err
	}

	for _, template := range c.Templates {
		templatePath := filepath.Join(dir, template.Name)

		if err := os.MkdirAll(filepath.Dir(templatePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(templatePath, template.Data, 0644); err != nil {
			return err
		}
	}

	for _, file := range c.Files {
		filePath := filepath.Join(dir, file.Name)

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filePath, file.Data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func lintChart(chartPath string) error {

	client := action.NewLint()

	result := client.Run([]string{chartPath}, nil)

	for _, message := range result.Messages {
		if message.Severity == support.ErrorSev {
			return fmt.Errorf("chart linting failed: %s", message.Err)
		}
	}

	return nil
}

func packageChart(chartPath string, w io.Writer) error {

	client := action.NewPackage()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	packagesDir := filepath.Join(homeDir, ".meshery", "tmp", "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create packages directory: %w", err)
	}

	client.Destination = packagesDir
	client.DependencyUpdate = false

	packagedChartPath, err := client.Run(chartPath, nil)
	if err != nil {
		return err
	}
	defer os.Remove(packagedChartPath)

	f, err := os.Open(packagedChartPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}
