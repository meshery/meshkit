package helm

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/meshery/meshkit/utils"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

func extractSemVer(versionConstraint string) string {
	reg := regexp.MustCompile(`v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+)?$`)
	match := reg.Find([]byte(versionConstraint))
	if match != nil {
		return string(match)
	}
	return ""
}

// DryRun a given helm chart to convert into k8s manifest
func DryRunHelmChart(chart *chart.Chart, kubernetesVersion string) ([]byte, error) {
	actconfig := new(action.Configuration)
	act := action.NewInstall(actconfig)
	act.ReleaseName = chart.Metadata.Name
	act.Namespace = "default"
	act.DryRun = true
	act.IncludeCRDs = true
	act.ClientOnly = true

	kubeVersion := kubernetesVersion
	if chart.Metadata.KubeVersion != "" {
		extractedVersion := extractSemVer(chart.Metadata.KubeVersion)

		if extractedVersion != "" {
			kubeVersion = extractedVersion
		}
	}

	if kubeVersion != "" {
		act.KubeVersion = &chartutil.KubeVersion{
			Version: kubeVersion,
		}
	}

	rel, err := act.Run(chart, nil)
	if err != nil {
		return nil, ErrDryRunHelmChart(err, chart.Name())
	}
	var manifests bytes.Buffer
	_, err = manifests.Write([]byte(strings.TrimSpace(rel.Manifest)))
	if err != nil {
		return nil, ErrDryRunHelmChart(err, chart.Name())
	}
	return manifests.Bytes(), nil
}

// Takes in the directory and converts HelmCharts/multiple manifests into a single K8s manifest
func ConvertToK8sManifest(path, kubeVersion string, w io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return utils.ErrReadDir(err, path)
	}
	helmChartPath := path
	if !info.IsDir() {
		helmChartPath, _ = strings.CutSuffix(path, filepath.Base(path))
	}
	if IsHelmChart(helmChartPath) {
		err := LoadHelmChart(helmChartPath, w, true, kubeVersion)
		if err != nil {
			return err
		}
		// If not a helm chart then assume k8s manifest.
		// Add introspection for compose files later on.
	} else if utils.IsYaml(path) {
		pathInfo, _ := os.Stat(path)
		if pathInfo.IsDir() {
			err := filepath.WalkDir(path, func(path string, d fs.DirEntry, _err error) error {
				err := writeToFile(w, path)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			err := writeToFile(w, path)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// writes in form of yaml files
func writeToFile(w io.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return utils.ErrReadFile(err, path)
	}

	var manifest map[string]any
	err = yaml.Unmarshal(data, &manifest)
	if err != nil {
		return utils.ErrWriteFile(err, path)
	}
	byt, err := yaml.Marshal(manifest)

	if err != nil {
		return utils.ErrWriteFile(err, path)
	}
	_, err = w.Write(byt)
	if err != nil {
		return utils.ErrWriteFile(err, path)
	}
	_, err = w.Write([]byte("\n---\n"))
	if err != nil {
		return utils.ErrWriteFile(err, path)
	}

	return nil
}

// Exisitence of Chart.yaml/Chart.yml indicates the directory contains a helm chart
func IsHelmChart(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, "Chart.yaml"))
	if err != nil {
		_, err = os.Stat(filepath.Join(dirPath, "Chart.yml"))
		if err != nil {
			return false
		}
	}
	return true
}

func LoadHelmChart(path string, w io.Writer, extractOnlyCrds bool, kubeVersion string) error {
	var errs []error
	chart, err := loader.Load(path)
	if err != nil {
		return ErrLoadHelmChart(err, path)
	}

	if !extractOnlyCrds {
		manifests, err := DryRunHelmChart(chart, kubeVersion)
		if err != nil {
			return ErrLoadHelmChart(err, path)
		}
		_, err = w.Write(manifests)
		return err
	}

	// Look for all the yaml file in the helm dir that is a CRD
	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return ErrLoadHelmChart(err, path)
		}
		if !d.IsDir() && (strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml")) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			if isCRDFile(data) {
				data = RemoveHelmPlaceholders(data)
				if err := writeToWriter(w, data); err != nil {
					errs = append(errs, err)
				}
			}
		}
		return nil
	})

	if err != nil {
		errs = append(errs, err)
	}

	return utils.CombineErrors(errs, "\n")
}

func writeToWriter(w io.Writer, data []byte) error {
	trimmedData := bytes.TrimSpace(data)

	if len(trimmedData) == 0 {
		return nil
	}

	// Check if the document already starts with separators
	startsWithSeparator := bytes.HasPrefix(trimmedData, []byte("---"))

	// If it doesn't start with ---, add one
	if !startsWithSeparator {
		if _, err := w.Write([]byte("---\n")); err != nil {
			return err
		}
	}

	if _, err := w.Write(trimmedData); err != nil {
		return err
	}

	_, err := w.Write([]byte("\n"))
	return err
}

// checks if the content is a CRD
// NOTE: kubernetes.IsCRD(manifest string) already exists however using that leads to cyclic dependency
func isCRDFile(content []byte) bool {
	str := string(content)
	return strings.Contains(str, "kind: CustomResourceDefinition")
}

// RemoveHelmPlaceholders - replaces helm templates placeholder with YAML compatible empty value
// since these templates cause YAML parsing error
// NOTE: this is a quick fix
func RemoveHelmPlaceholders(data []byte) []byte {
	content := string(data)

	// Regular expressions to match different Helm template patterns
	// Match multiline template blocks that start with {{- and end with }}
	multilineRegex := regexp.MustCompile(`(?s){{-?\s*.*?\s*}}`)

	// Match single line template expressions
	singleLineRegex := regexp.MustCompile(`{{-?\s*[^}]*}}`)

	// Process the content line by line to maintain YAML structure
	lines := strings.Split(content, "\n")
	var processedLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			processedLines = append(processedLines, line)
			continue
		}

		// Handle multiline template blocks first
		if multilineRegex.MatchString(line) {
			// If line starts with indentation + list marker
			if listMatch := regexp.MustCompile(`^(\s*)- `).FindStringSubmatch(line); listMatch != nil {
				// Convert list item to empty map to maintain structure
				processedLines = append(processedLines, listMatch[1]+"- {}")
				continue
			}

			// If it's a value assignment with multiline template
			if valueMatch := regexp.MustCompile(`^(\s*)(\w+):\s*{{`).FindStringSubmatch(line); valueMatch != nil {
				// Preserve the key with empty map value
				processedLines = append(processedLines, valueMatch[1]+valueMatch[2]+": {}")
				continue
			}

			// For other multiline templates, replace with empty line
			processedLines = append(processedLines, "")
			continue
		}

		// Handle single line template expressions
		if singleLineRegex.MatchString(line) {
			// If line contains a key-value pair
			if keyMatch := regexp.MustCompile(`^(\s*)(\w+):\s*{{`).FindStringSubmatch(line); keyMatch != nil {
				// Preserve the key with empty string value
				processedLines = append(processedLines, keyMatch[1]+keyMatch[2]+": ")
				continue
			}

			// If line is a list item
			if listMatch := regexp.MustCompile(`^(\s*)- `).FindStringSubmatch(line); listMatch != nil {
				// Convert to empty map to maintain list structure
				processedLines = append(processedLines, listMatch[1]+"- {}")
				continue
			}

			// For standalone template expressions, remove them (includes, control statements)
			line = singleLineRegex.ReplaceAllString(line, "")
			if strings.TrimSpace(line) != "" {
				processedLines = append(processedLines, line)
			}
			continue
		}

		processedLines = append(processedLines, line)
	}

	return []byte(strings.Join(processedLines, "\n"))
}

// SanitizeHelmName - sanitizes the name of the helm chart
// Helm chart names must be lowercase and can only contain alphanumeric characters, dashes, and underscores
// Example: "My Chart" -> "my-chart"
func SanitizeHelmName(name string) string {
	if name == "" {
		return "meshery-design"
	}

	result := strings.ToLower(name)
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	result = reg.ReplaceAllString(result, "-")

	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	result = strings.Trim(result, "-")

	if result == "" {
		return "meshery-design"
	}

	const maxLength = 40
	if len(result) > maxLength {
		result = result[:maxLength]

		result = strings.Trim(result, "-")
	}

	return result
}
