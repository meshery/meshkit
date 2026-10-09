package kubernetes

import (
	"os"

	"helm.sh/helm/v3/pkg/chart/loader"
)

func GetManifestsFromHelm(url string) (string, error) {
	downloadDir, err := os.MkdirTemp("", "meshkit-helm-manifest-")
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}
	defer os.RemoveAll(downloadDir)

	chartLocation, err := fetchHelmChart(url, downloadDir)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}

	chart, err := loader.Load(chartLocation)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}
	manifests := ""
	for _, crdobject := range chart.CRDObjects() {
		manifests += "\n---\n"
		manifests += string(crdobject.File.Data)
	}
	return manifests, nil
}
