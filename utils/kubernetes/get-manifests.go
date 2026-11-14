package kubernetes

import (
	"fmt"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"sort"
	"strings"
)

// GetManifestsFromHelm fetches the chart, loads it, and renders templates + CRDs
func GetManifestsFromHelm(url string) (string, error) {
	chartLocation, err := fetchHelmChart(url, "")
	if err != nil {
		return "", ErrApplyHelmChart(fmt.Errorf("failed to fetch helm chart: %w", err))
	}

	chart, err := loader.Load(chartLocation)
	if err != nil {
		return "", ErrApplyHelmChart(fmt.Errorf("failed to load chart from %s: %w", chartLocation, err))
	}

	releaseOptions := chartutil.ReleaseOptions{
		Name:      "meshery-helm-release",
		Namespace: "default",
		Revision:  1,
		IsInstall: true,
	}

	caps := chartutil.DefaultCapabilities

	values, err := chartutil.ToRenderValues(chart, chartutil.Values{}, releaseOptions, caps)
	if err != nil {
		return "", ErrApplyHelmChart(fmt.Errorf("failed to generate render values: %w", err))
	}

	renderedFiles, err := engine.Render(chart, values)
	if err != nil {
		return "", ErrApplyHelmChart(fmt.Errorf("failed to render chart templates: %w", err))
	}

	var b strings.Builder

	// Helper to safely append separators
	appendSeparator := func() {
		if b.Len() > 0 {
			// 1. Check if the buffer ends with a newline.
			// 2. If not, add one to ensure "---" starts on a fresh line.
			if !strings.HasSuffix(b.String(), "\n") {
				b.WriteString("\n")
			}
			b.WriteString("---\n")
		}
	}
	//  Append CRDs
	for _, crdobject := range chart.CRDObjects() {
		appendSeparator()
		b.Write(crdobject.File.Data)
	}

	keys := make([]string, 0, len(renderedFiles))
	for k := range renderedFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	//  Append Rendered Templates
	for _, filename := range keys {
		fileContent := renderedFiles[filename]
		// Filter out non-manifest files
		if strings.HasSuffix(filename, "NOTES.txt") || strings.Contains(filename, "/tests/") {
			continue
		}
		if strings.TrimSpace(fileContent) == "" {
			continue
		}

		appendSeparator()
		b.WriteString(fileContent)
	}

	manifests := b.String()

	if strings.TrimSpace(manifests) == "" {
		return "", ErrApplyHelmChart(fmt.Errorf("chart rendered empty manifests"))
	}

	return manifests, nil
}
