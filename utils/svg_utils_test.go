package utils

import (
	"strings"
	"testing"
)

func TestUpdateSVGString_BasicSVG(t *testing.T) {
	svg := `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><rect width="50" height="50"/></svg>`
	result, err := UpdateSVGString(svg, 200, 200, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, XMLTAG) {
		t.Error("expected XML header in result")
	}
}

func TestUpdateSVGString_SkipHeader(t *testing.T) {
	svg := `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><rect/></svg>`
	result, err := UpdateSVGString(svg, 200, 200, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, XMLTAG) {
		t.Error("expected no XML header when skipHeader=true")
	}
}

func TestUpdateSVGString_NoWidthHeight(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`
	result, err := UpdateSVGString(svg, 300, 400, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestUpdateSVGString_EmptyString(t *testing.T) {
	result, err := UpdateSVGString("", 100, 100, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty input produces empty output (no XML header added)
	if result != "" {
		t.Errorf("expected empty result for empty input, got %q", result)
	}
}

func TestUpdateSVGString_InvalidXML(t *testing.T) {
	svg := `<svg><not closed`
	_, err := UpdateSVGString(svg, 100, 100, false)
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}
