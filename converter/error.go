package converter

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	// Error codes for Helm Converter
	ErrLoadPatternCode = "meshkit-11315"
	ErrConvertK8sCode  = "meshkit-11316"
	ErrCreateChartCode = "meshkit-11317"
	ErrHelmPackageCode = "meshkit-11318"
)

// ErrLoadPattern returns error for failing to load pattern file
func ErrLoadPattern(err error, patternFile string) error {
	return errors.New(ErrLoadPatternCode,
		errors.Critical,
		[]string{"Failed to load pattern file"},
		[]string{fmt.Sprintf("Error loading pattern file '%s': %s", patternFile, err.Error())},
		[]string{"The pattern file might be invalid or inaccessible"},
		[]string{"Verify the pattern file exists and has correct format"})
}

// ErrConvertK8s returns error for failing to convert to K8s
func ErrConvertK8s(err error) error {
	return errors.New(ErrConvertK8sCode,
		errors.Critical,
		[]string{"Failed to convert to Kubernetes manifest"},
		[]string{err.Error()},
		[]string{"The pattern might contain incompatible elements"},
		[]string{"Verify the pattern content is valid for Kubernetes"})
}

// ErrCreateHelmChart returns error for failing to create Helm chart
func ErrCreateHelmChart(err error, operation string) error {
	return errors.New(ErrCreateChartCode,
		errors.Critical,
		[]string{"Failed to create Helm chart"},
		[]string{fmt.Sprintf("Error during operation '%s': %s", operation, err.Error())},
		[]string{"File system permissions or disk space issues"},
		[]string{"Check permissions and available disk space"})
}

// ErrHelmPackage returns error for failing to package Helm chart
func ErrHelmPackage(err error) error {
	return errors.New(ErrHelmPackageCode,
		errors.Critical,
		[]string{"Helm packaging failed"},
		[]string{err.Error()},
		[]string{"Issues with the Helm chart structure or configuration"},
		[]string{"Verify the chart structure is valid for Helm"})
}

//TODO: Add error handling functions for k8s manifest converter
