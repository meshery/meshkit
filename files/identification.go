package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	// appsv1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

const MESHERY_DESIGN = "Design"
const KUBERNETES_MANIFEST = "KubernetesManifest"
const DOCKER_COMPOSE = "DockerCompose"
const KUSTOMIZATION = "Kustomization"
const HELM_CHART = "HelmChart"
const MESHERY_VIEW = "View"

type IdentifiedFile struct {
	Type string
	// pattern.PatternFile,[]runtime.Object ,*chart.Chart,resmap etc
	ParsedFile interface{}
}

func IdentifyFile(sanitizedFile SanitizedFile) (IdentifiedFile, error) {
	var err error
	var parsed interface{}

	if parsed, err = ParseFileAsKustomization(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       KUSTOMIZATION,
			ParsedFile: parsed,
		}, nil
	}

	fmt.Println(fmt.Errorf("Identify Kustomize errors %w", err))

	if parsed, err = ParseFileAsMesheryDesign(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       MESHERY_DESIGN,
			ParsedFile: parsed,
		}, nil
	}

	if parsed, err = ParseFileAsKubernetesManifest(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       KUBERNETES_MANIFEST,
			ParsedFile: parsed,
		}, nil
	}

	if parsed, err = ParseFileAsHelmChart(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       HELM_CHART,
			ParsedFile: parsed,
		}, nil
	}

	return IdentifiedFile{}, fmt.Errorf("Unsupported FileType %w", err)
}

func ParseFileAsMesheryDesign(file SanitizedFile) (pattern.PatternFile, error) {

	var parsed pattern.PatternFile

	switch file.FileExt {

	case ".yml", ".yaml":

		decoder := yaml.NewDecoder(bytes.NewReader(file.RawData))
		decoder.KnownFields(true)
		err := decoder.Decode(&parsed)
		return parsed, err

	case ".json":

		decoder := json.NewDecoder(bytes.NewReader(file.RawData))

		decoder.DisallowUnknownFields()
		err := decoder.Decode(&parsed)
		return parsed, err

	default:
		return pattern.PatternFile{}, fmt.Errorf("Invalid File extension %s", file.FileExt)
	}

}

func ParseFileAsKubernetesManifest(file SanitizedFile) ([]runtime.Object, error) {
	// Normalize file extension to lowercase
	fileExt := strings.ToLower(file.FileExt)

	// Check if the file extension is valid
	if fileExt != ".yml" && fileExt != ".yaml" {
		return nil, fmt.Errorf("invalid file extension: %s, only .yml and .yaml are supported", file.FileExt)
	}

	// Initialize the scheme with the core Kubernetes types
	// This should be done once, typically in an init() function or a global variable
	// clientgoscheme.AddToScheme(scheme.Scheme)

	// Create a decoder
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode

	// Create a YAML decoder for the multi-document YAML file
	yamlDecoder := k8sYaml.NewYAMLOrJSONDecoder(strings.NewReader(string(file.RawData)), 4096)

	var objects []runtime.Object

	// Decode each document in the YAML file
	for {
		var raw runtime.RawExtension
		if err := yamlDecoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break // End of file
			}
			return nil, fmt.Errorf("failed to decode YAML document: %v", err)
		}

		if len(raw.Raw) == 0 {
			continue // Skip empty documents
		}

		// Decode the raw YAML into a runtime.Object
		obj, _, err := decode(raw.Raw, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decode YAML into Kubernetes object: %v", err)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// findChartDir uses filepath.Glob to locate Chart.yaml in nested directories
func FindChartDir(root string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "**/Chart.yaml"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no valid Helm chart found in %s", root)
	}

	// Extract directory path where Chart.yaml is found
	return filepath.Dir(matches[0]), nil
}

var ValidHelmChartFileExtensions = map[string]bool{
	".tar":    true,
	".tgz":    true,
	".gz":     true,
	".tar.gz": true,
}

var ValidKustomizeFileExtensions = map[string]bool{
	".yml":    true, // single kustomization.yml file
	".yaml":   true,
	".tar":    true,
	".tgz":    true,
	".gz":     true,
	".tar.gz": true,
}

// ParseFileAsHelmChart loads a Helm chart from the extracted directory.
func ParseFileAsHelmChart(file SanitizedFile) (*chart.Chart, error) {

	if !ValidHelmChartFileExtensions[file.FileExt] {
		return nil, fmt.Errorf("Invalid file extension %s", file.FileExt)
	}

	// Use Helm's loader.LoadDir to load the chart
	// This function automatically handles nested directories and locates Chart.yaml
	chart, err := loader.LoadArchive(bytes.NewReader(file.RawData))
	if err != nil {
		return nil, fmt.Errorf("failed to load Helm chart  %v", err)
	}

	// Validate the chart (optional but recommended)
	if err := chart.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Helm chart: %v", err)
	}

	return chart, nil
}

// ParseFileAsKustomization processes a sanitized file and returns a Kustomize ResMap
func ParseFileAsKustomization(file SanitizedFile) (resmap.ResMap, error) {
	// Validate file extension

	if !ValidKustomizeFileExtensions[file.FileExt] {
		return nil, fmt.Errorf("invalid file extension %s", file.FileExt)
	}

	var fs filesys.FileSystem
	var path string

	if file.ExtractedContent != nil {
		// Check if ExtractedContent is a directory
		// If it's a directory, use it directly with MakeFsOnDisk
		fs = filesys.MakeFsOnDisk()
		path = file.ExtractedContent.Name()

		// Ensure the path points to the directory containing kustomization.yaml
		kustomizationPath := filepath.Join(path, "kustomization.yaml")
		if _, err := os.Stat(kustomizationPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("kustomization.yaml not found in extracted directory")
		}

	} else {
		// ExtractedContent is nil → Read from file.RawData (single-file case)
		if len(file.RawData) == 0 {
			return nil, fmt.Errorf("file is empty or not extracted")
		}

		fs = filesys.MakeFsInMemory()
		path = "/kustomization.yaml"
		err := fs.WriteFile(path, file.RawData)
		if err != nil {
			return nil, fmt.Errorf("failed to write raw data to in-memory filesystem: %v", err)
		}
	}

	// Use krusty to build the Kustomize resources
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := k.Run(fs, path)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kustomize resources: %v", err)
	}

	return resMap, nil
}
