package github

import (
	"net/url"
	"testing"
)

func TestGetVersion(t *testing.T) {
	gitProtoString := "git://github.com/kubernetes-sigs/metrics-server/master/charts"
	parsedUrl, _ := url.Parse(gitProtoString)
	testData := NewDownloaderForScheme("git", parsedUrl, "metrics-server")
	output, _ := testData.GetContent()

	if output == nil {
		t.Fatalf("Expected non-nil data, got nil")
	}

	if version := output.GetVersion(); version != "metrics-server-helm-chart-3.12.1" {
		t.Errorf("Expected version metrics-server-helm-chart-3.12.1, got %s", version)
	}
}
