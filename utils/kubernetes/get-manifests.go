package kubernetes

import (
	"helm.sh/helm/v3/pkg/chart/loader"
)

func GetManifestsFromHelm(url string) (string, error) {
	chartLocation, err := fetchHelmChart(url)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}

	chart, err := loader.Load(chartLocation)
	if err != nil {
		return "", ErrApplyHelmChart(err)
	}
	var manifests string = ""
	for _, crdobject := range chart.CRDObjects() {
		manifests += "\n---\n"
		manifests += string(crdobject.File.Data)
	}
	return manifests, nil
}
