package helm

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/layer5io/meshkit/encoding"
	"github.com/layer5io/meshkit/utils"
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

	byt, err := encoding.ToYaml(data)
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
	if extractOnlyCrds {
		crds := chart.CRDObjects()
		size := len(crds)
		for index, crd := range crds {
			_, err := w.Write(crd.File.Data)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if index == size-1 {
				break
			}
			_, _ = w.Write([]byte("\n---\n"))
		}
	} else {
		manifests, err := DryRunHelmChart(chart, kubeVersion)
		if err != nil {
			return ErrLoadHelmChart(err, path)
		}
		_, err = w.Write(manifests)
		if err != nil {
			return ErrLoadHelmChart(err, path)
		}
	}
	return utils.CombineErrors(errs, "\n")
}
