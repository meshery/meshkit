package github

import (
	"net/url"
	"testing"
)

func TestURL_GetVersion(t *testing.T) {
	UrlString := "https://github.com/kubernetes-sigs/metrics-server"
	parsedUrl, _ := url.Parse(UrlString)
	testData := NewDownloaderForScheme("https", parsedUrl, "metrics-server")
	output, _ := testData.GetContent()

	if output == nil {
		t.Fatalf("Expected non-nil data, got nil")
	}

	if version := output.GetVersion(); version != "metrics-server-helm-chart-3.12.1" {
		t.Errorf("Expected version metrics-server-helm-chart-3.12.1, got %s", version)
	}
}
