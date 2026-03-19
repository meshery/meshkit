package converter

import (
	"testing"
)

func TestNewFormatConverter_K8sManifest(t *testing.T) {
	conv, err := NewFormatConverter(K8sManifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conv == nil {
		t.Fatal("expected non-nil converter")
	}
}

func TestNewFormatConverter_HelmChart(t *testing.T) {
	conv, err := NewFormatConverter(HelmChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conv == nil {
		t.Fatal("expected non-nil converter")
	}
}

func TestNewFormatConverter_UnsupportedFormat(t *testing.T) {
	_, err := NewFormatConverter(DockerCompose)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestNewFormatConverter_Design(t *testing.T) {
	_, err := NewFormatConverter(Design)
	if err == nil {
		t.Fatal("expected error for Design format")
	}
}

func TestNewFormatConverter_UnknownFormat(t *testing.T) {
	_, err := NewFormatConverter(DesignFormat("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestDesignFormatConstants(t *testing.T) {
	if HelmChart != "helm-chart" {
		t.Errorf("expected helm-chart, got %s", HelmChart)
	}
	if DockerCompose != "Docker Compose" {
		t.Errorf("expected Docker Compose, got %s", DockerCompose)
	}
	if K8sManifest != "Kubernetes Manifest" {
		t.Errorf("expected Kubernetes Manifest, got %s", K8sManifest)
	}
	if Design != "Design" {
		t.Errorf("expected Design, got %s", Design)
	}
}
