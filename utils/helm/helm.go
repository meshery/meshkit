package helm

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// DryRun a given helm chart to convert into k8s manifest
func DryRunHelmChart(chart *chart.Chart) ([]byte, error) {
	actconfig := new(action.Configuration)
	act := action.NewInstall(actconfig)
	act.ReleaseName = "dry-run-release"
	act.CreateNamespace = true
	act.Namespace = "default"
	act.DryRun = true
	act.IncludeCRDs = true
	act.ClientOnly = true
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
func ConvertToK8sManifest(path string, w io.Writer) error {
	info, _ := os.Stat(path)
	helmChartPath := path
	if !info.IsDir() {
		helmChartPath, _ = strings.CutSuffix(path, filepath.Base(path))
	}
	if IsHelmChart(helmChartPath) {
		err := LoadHelmChart(helmChartPath, w, true)
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
	_, err = w.Write(data)
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

func LoadHelmChart(path string, w io.Writer, extractOnlyCrds bool) error {
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
		manifests, err := DryRunHelmChart(chart)
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
