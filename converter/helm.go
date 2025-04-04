package converter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/layer5io/meshkit/models/patterns"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
)

type HelmConverter struct{}

func (h *HelmConverter) Convert(patternFile string) (string, error) {
	pattern, err := patterns.GetPatternFormat(patternFile)
	if err != nil {
		return "", err
	}
	patterns.ProcessAnnotations(pattern)

	tmpDir, err := os.MkdirTemp("", "meshery-helm-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	chartName := sanitizeHelmName(pattern.Name)
	chartDir := filepath.Join(tmpDir, chartName)
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return "", err
	}

	err = generateHelmChart(pattern, chartDir)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == tmpDir {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		return "", err
	}

	if err := gw.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func generateHelmChart(patternFile *pattern.PatternFile, chartDir string) error {
	chartsDir := filepath.Join(chartDir, "charts")
	templatesDir := filepath.Join(chartDir, "templates")

	for _, dir := range []string{chartsDir, templatesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	chartYaml := map[string]interface{}{
		"apiVersion":  "v2",
		"name":        sanitizeHelmName(patternFile.Name),
		"description": fmt.Sprintf("Helm chart for Meshery design %s", patternFile.Name),
		"type":        "application",
		"version":     "0.1.0",
		"appVersion":  patternFile.Version,
		"keywords":    []string{"meshery", "design", "kubernetes"},
		"home":        "https://meshery.io/",
		"sources":     []string{"https://github.com/meshery/meshery"},
		"maintainers": []map[string]string{
			{
				"name":  "Meshery Authors",
				"email": "maintainers@meshery.io",
			},
		},
	}

	dependencies := extractDependencies(patternFile)
	if len(dependencies) > 0 {
		chartYaml["dependencies"] = dependencies
	}

	chartYamlData, err := yaml.Marshal(chartYaml)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), chartYamlData, 0644); err != nil {
		return err
	}

	values, err := generateValuesYaml(patternFile)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(values), 0644); err != nil {
		return err
	}

	templatesByKind, err := generateTemplates(patternFile, templatesDir)
	if err != nil {
		return err
	}

	helpersContent := generateHelperTemplates()
	if err := os.WriteFile(filepath.Join(templatesDir, "_helpers.tpl"), []byte(helpersContent), 0644); err != nil {
		return err
	}

	notesContent := generateNotes(patternFile, templatesByKind)
	if err := os.WriteFile(filepath.Join(templatesDir, "NOTES.txt"), []byte(notesContent), 0644); err != nil {
		return err
	}

	readmeContent := generateReadme(patternFile, templatesByKind)
	if err := os.WriteFile(filepath.Join(chartDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		return err
	}

	return nil
}

func extractDependencies(patternFile *pattern.PatternFile) []map[string]interface{} {
	dependencies := []map[string]interface{}{}
	addedDeps := make(map[string]bool)

	for _, comp := range patternFile.Components {
		if comp.Model.Id.String() == "" || comp.Model.Registrant.Id.String() == "" || comp.Model.Registrant.Kind == "" {
			continue
		}

		if comp.Model.Registrant.Kind == "artifacthub" {
			safeName := sanitizeHelmName(comp.Model.Name)
			if addedDeps[safeName] {
				continue
			}

			if safeName == "kubernetes" {
				safeName = "kubernetes-charts"
			}

			addedDeps[safeName] = true
			alias := fmt.Sprintf("%s-%s", safeName, sanitizeHelmName(comp.DisplayName))

			dependencies = append(dependencies, map[string]interface{}{
				"name":       safeName,
				"version":    comp.Model.Model.Version,
				"repository": fmt.Sprintf("https://artifacthub.io/packages/helm/%s/%s", safeName, safeName),
				"condition":  fmt.Sprintf("components.%s.enabled", sanitizeHelmName(comp.DisplayName)),
				"alias":      alias,
			})
		}
	}

	return dependencies
}

func generateValuesYaml(patternFile *pattern.PatternFile) (string, error) {
	values := map[string]interface{}{
		"global": map[string]interface{}{
			"namespace": "default",
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "Meshery",
			},
		},
		"components": map[string]interface{}{},
	}

	componentsValues := values["components"].(map[string]interface{})

	for _, comp := range patternFile.Components {
		safeCompName := sanitizeHelmName(comp.DisplayName)

		compValues := map[string]interface{}{
			"enabled": true,
			"image": map[string]interface{}{
				"repository": "layer5/meshery",
				"tag":        "stable-latest",
				"pullPolicy": "Always",
			},
		}

		if comp.Component.Kind == "Deployment" || comp.Component.Kind == "StatefulSet" {
			replicas := 1
			if conf, ok := comp.Configuration["spec"]; ok {
				if spec, ok := conf.(map[string]interface{}); ok {
					if r, ok := spec["replicas"]; ok {
						if rVal, ok := r.(int); ok {
							replicas = rVal
						}
					}
				}
			}

			compValues["replicas"] = replicas

			if comp.Configuration != nil {
				if spec, ok := comp.Configuration["spec"].(map[string]interface{}); ok {
					if template, ok := spec["template"].(map[string]interface{}); ok {
						if podSpec, ok := template["spec"].(map[string]interface{}); ok {
							if containers, ok := podSpec["containers"].([]interface{}); ok && len(containers) > 0 {
								if container, ok := containers[0].(map[string]interface{}); ok {
									if image, ok := container["image"].(string); ok {
										imageParts := strings.Split(image, ":")
										repository := imageParts[0]
										tag := "latest"
										if len(imageParts) > 1 {
											tag = imageParts[1]
										}

										imageConfig := compValues["image"].(map[string]interface{})
										imageConfig["repository"] = repository
										imageConfig["tag"] = tag
									}
								}
							}
						}
					}
				}
			}
		}

		if conf, ok := comp.Configuration["metadata"]; ok {
			if metadata, ok := conf.(map[string]interface{}); ok {
				if namespace, ok := metadata["namespace"]; ok && namespace != "default" {
					compValues["namespace"] = namespace
				}
			}
		}

		componentsValues[safeCompName] = compValues
	}

	var buf bytes.Buffer
	buf.WriteString("# Default values for Meshery design\n")
	buf.WriteString("# This is a YAML-formatted file.\n")
	buf.WriteString("# Declare variables to be passed into your templates.\n\n")

	yamlData, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}

	buf.Write(yamlData)
	return buf.String(), nil
}

func generateTemplates(patternFile *pattern.PatternFile, templatesDir string) (map[string][]string, error) {
	templatesByKind := make(map[string][]string)

	for i, comp := range patternFile.Components {
		if comp.Component.Kind == "" {
			continue
		}

		k8sResource := createK8sResource(comp)
		addHelmTemplate(k8sResource, comp)

		resourceYaml, err := yaml.Marshal(k8sResource)
		if err != nil {
			return nil, err
		}

		// Use index function for accessing hyphenated properties
		templateContent := fmt.Sprintf(`{{- if index .Values.components "%s" "enabled" }}
%s
{{- end }}
`, sanitizeHelmName(comp.DisplayName), string(resourceYaml))

		kind := strings.ToLower(comp.Component.Kind)
		name := sanitizeFileName(comp.DisplayName)
		filename := fmt.Sprintf("%02d-%s-%s.yaml", i+1, kind, name)

		if _, ok := templatesByKind[kind]; !ok {
			templatesByKind[kind] = []string{}
		}
		templatesByKind[kind] = append(templatesByKind[kind], name)

		if err := os.WriteFile(filepath.Join(templatesDir, filename), []byte(templateContent), 0644); err != nil {
			return nil, err
		}
	}

	return templatesByKind, nil
}

func createK8sResource(comp *component.ComponentDefinition) map[string]interface{} {
	// Initialize with default structure
	k8sResource := map[string]interface{}{
		"apiVersion": comp.Component.Version,
		"kind":       comp.Component.Kind,
		"metadata": map[string]interface{}{
			"name": comp.DisplayName,
			// Use index function for accessing hyphenated properties
			"namespace": fmt.Sprintf("{{ index .Values.components \"%s\" \"namespace\" | default .Values.global.namespace }}",
				sanitizeHelmName(comp.DisplayName)),
			"labels": map[string]interface{}{
				"app.kubernetes.io/name":       comp.DisplayName,
				"app.kubernetes.io/instance":   "{{ .Release.Name }}",
				"app.kubernetes.io/managed-by": "{{ .Release.Service }}",
				"helm.sh/chart":                "{{ include \"chart.name\" . }}-{{ .Chart.Version }}",
			},
		},
	}

	if conf, ok := comp.Configuration["metadata"]; ok {
		if metadata, ok := conf.(map[string]interface{}); ok {
			if annotations, ok := metadata["annotations"]; ok {
				k8sResource["metadata"].(map[string]interface{})["annotations"] = annotations
			}
			if labels, ok := metadata["labels"]; ok {
				for k, v := range labels.(map[string]interface{}) {
					k8sResource["metadata"].(map[string]interface{})["labels"].(map[string]interface{})[k] = v
				}
			}
		}
	}

	for k, v := range comp.Configuration {
		if k != "apiVersion" && k != "kind" && k != "metadata" {
			k8sResource[k] = processConfigValue(v, comp.DisplayName, comp.Component.Kind)
		}
	}

	return k8sResource
}

func processConfigValue(value interface{}, componentName, kind string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = processConfigValue(val, componentName, kind)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = processConfigValue(val, componentName, kind)
		}
		return result

	case string:
		if kind == "Deployment" && strings.Contains(v, ":") && strings.HasPrefix(v, "layer5/") {
			safeName := sanitizeHelmName(componentName)
			// Use index function for accessing hyphenated properties
			return fmt.Sprintf("{{ index .Values.components \"%s\" \"image\" \"repository\" | default \"%s\" }}:{{ index .Values.components \"%s\" \"image\" \"tag\" | default \"latest\" }}",
				safeName, strings.Split(v, ":")[0], safeName)
		}
		return v

	default:
		return v
	}
}

func addHelmTemplate(resource map[string]interface{}, comp *component.ComponentDefinition) {
	safeName := sanitizeHelmName(comp.DisplayName)

	switch comp.Component.Kind {
	case "Deployment", "StatefulSet":
		if spec, ok := resource["spec"].(map[string]interface{}); ok {
			// Use index function for accessing hyphenated properties
			spec["replicas"] = fmt.Sprintf("{{ index .Values.components \"%s\" \"replicas\" }}", safeName)
		}

	case "Service":
		if spec, ok := resource["spec"].(map[string]interface{}); ok {
			if selector, ok := spec["selector"].(map[string]interface{}); ok {
				for k := range selector {
					selector[k] = fmt.Sprintf("{{ include \"helpers.selectorLabels\" . }}")
				}
			}
		}
	}
}

func generateHelperTemplates() string {
	return `{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
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

{{/*
Create common labels
*/}}
{{- define "helpers.labels" -}}
helm.sh/chart: {{ include "chart.name" . }}-{{ .Chart.Version }}
{{ include "helpers.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
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
	readme.WriteString(fmt.Sprintf("helm install my-release ./%s\n", sanitizeHelmName(patternFile.Name)))
	readme.WriteString("```\n\n")

	readme.WriteString("## Configuration\n\n")
	readme.WriteString("The following table lists the configurable parameters of the chart and their default values.\n\n")
	readme.WriteString("| Parameter | Description | Default |\n")
	readme.WriteString("| --- | --- | --- |\n")
	readme.WriteString("| `global.namespace` | Default namespace for all components | `default` |\n")

	for _, comp := range patternFile.Components {
		safeName := sanitizeHelmName(comp.DisplayName)
		readme.WriteString(fmt.Sprintf("| `components.%s.enabled` | Enable %s component | `true` |\n",
			safeName, comp.DisplayName))

		if comp.Component.Kind == "Deployment" || comp.Component.Kind == "StatefulSet" {
			readme.WriteString(fmt.Sprintf("| `components.%s.replicas` | Number of replicas for %s | `1` |\n",
				safeName, comp.DisplayName))
		}
	}

	readme.WriteString("\n## Components\n\n")
	for kind, names := range templatesByKind {
		titleKind := strings.Title(kind)
		readme.WriteString(fmt.Sprintf("### %s\n\n", titleKind))
		for _, name := range names {
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
