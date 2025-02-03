package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/encoding"
	"github.com/layer5io/meshkit/models/oci"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes/kompose"
	"github.com/layer5io/meshkit/utils/walker"
	"github.com/meshery/schemas/models/core"
	"github.com/meshery/schemas/models/v1beta1/pattern"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	dockerLoader "github.com/docker/cli/cli/compose/loader"
	dockerTypes "github.com/docker/cli/cli/compose/types"
)

type IdentifiedFile struct {
	Type string

	// pattern.PatternFile (meshery-design),
	// []runtime.Object (k8s manifest) ,
	// *chart.Chart (helm-chart),
	// resmap.ResMap (kustomize),
	// dockerTypes.Config (docker-compose) etc
	ParsedFile interface{}
}

func IdentifyFile(sanitizedFile SanitizedFile) (IdentifiedFile, error) {
	var err error
	var parsed interface{}

	// Map to store identification errors for each file type
	identificationErrorsTrace := map[string]error{}

	// Attempt to parse the file as a Meshery design
	if parsed, err = ParseFileAsMesheryDesign(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       core.IacFileTypes.MESHERY_DESIGN,
			ParsedFile: parsed,
		}, nil
	}
	identificationErrorsTrace[core.IacFileTypes.MESHERY_DESIGN] = err

	// Attempt to parse the file as a Kubernetes manifest
	if parsed, err = ParseFileAsKubernetesManifest(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       core.IacFileTypes.KUBERNETES_MANIFEST,
			ParsedFile: parsed,
		}, nil
	}
	identificationErrorsTrace[core.IacFileTypes.KUBERNETES_MANIFEST] = err

	// Attempt to parse the file as a Helm chart
	if parsed, err = ParseFileAsHelmChart(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       core.IacFileTypes.HELM_CHART,
			ParsedFile: parsed,
		}, nil
	}
	identificationErrorsTrace[core.IacFileTypes.HELM_CHART] = err

	// Attempt to parse the file as a Docker Compose file
	if parsed, err = ParseFileAsDockerCompose(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       core.IacFileTypes.DOCKER_COMPOSE,
			ParsedFile: parsed,
		}, nil
	}
	identificationErrorsTrace[core.IacFileTypes.DOCKER_COMPOSE] = err

	// Attempt to parse the file as a Kustomization file
	if parsed, err = ParseFileAsKustomization(sanitizedFile); err == nil {
		return IdentifiedFile{
			Type:       core.IacFileTypes.KUSTOMIZE,
			ParsedFile: parsed,
		}, nil
	}
	identificationErrorsTrace[core.IacFileTypes.KUSTOMIZE] = err

	// If no file type matched, return a detailed error with the identification trace
	return IdentifiedFile{}, ErrFailedToIdentifyFile(sanitizedFile.FileName, sanitizedFile.FileExt, identificationErrorsTrace)
}

func ParseCompressedOCIArtifactIntoDesign(artifact []byte) (*pattern.PatternFile, error) {

	// Assume design is in OCI Tarball Format
	tmpDir, err := oci.CreateTempOCIContentDir()
	if err != nil {
		return nil, utils.ErrCreateDir(err, "OCI")
	}
	defer os.RemoveAll(tmpDir)

	tmpInputDesignFile := filepath.Join(tmpDir, "design.tar")
	file, err := os.Create(tmpInputDesignFile)
	if err != nil {
		return nil, utils.ErrCreateFile(err, tmpInputDesignFile)
	}
	defer file.Close()

	reader := bytes.NewReader(artifact)
	if _, err := io.Copy(file, reader); err != nil {
		return nil, utils.ErrWritingIntoFile(err, tmpInputDesignFile)
	}

	tmpOutputDesignFile := filepath.Join(tmpDir, "output")
	// Extract the tarball
	if err := oci.UnCompressOCIArtifact(tmpInputDesignFile, tmpOutputDesignFile); err != nil {
		return nil, ErrUnCompressOCIArtifact(err)
	}

	files, err := walker.WalkLocalDirectory(tmpOutputDesignFile)
	if err != nil {
		return nil, ErrWaklingLocalDirectory(err)
	}

	// TODO: Add support to merge multiple designs into one
	// Currently, assumes to save only the first design
	if len(files) == 0 {
		return nil, ErrEmptyOCIImage(fmt.Errorf("no design file detected in the imported OCI image"))
	}
	var design *walker.File

	// the extracted layer may contain metadata files like artifact.yml for artifacthub,etc
	for _, file := range files {
		if file.Name == "design.yml" {
			design = file
		}
	}

	if design == nil {
		return nil, ErrEmptyOCIImage(fmt.Errorf("No design file detected in the imported OCI image"))
	}

	var patternFile pattern.PatternFile

	err = encoding.Unmarshal([]byte(design.Content), &patternFile)

	if err != nil {
		return nil, ErrDecodePattern(err)
	}
	patternFile.Name = design.Name

	return &patternFile, nil
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

	case ".tgz", ".tar", ".tar.gz", ".zip": // try to parse oci artifacts
		parsed_design, err := ParseCompressedOCIArtifactIntoDesign(file.RawData)
		if parsed_design == nil || err != nil {
			return pattern.PatternFile{}, err
		}

		return *parsed_design, err

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
			// Fallback: Convert to Unstructured object for unknown API types
			unstructuredObj := &unstructured.Unstructured{}
			if err := json.Unmarshal(raw.Raw, unstructuredObj); err != nil {
				return nil, fmt.Errorf("failed to decode YAML into Kubernetes object: %v", err)
			}
			objects = append(objects, unstructuredObj)
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
	".zip":    true,
}

var ValidKustomizeFileExtensions = map[string]bool{
	".yml":    true, // single kustomization.yml file
	".yaml":   true,
	".tar":    true,
	".tgz":    true,
	".gz":     true,
	".tar.gz": true,
	".zip":    true,
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

	if file.ExtractedContentPath != "" {
		path = file.ExtractedContentPath
		// Check if ExtractedContent is a directory
		// If it's a directory, use it directly with MakeFsOnDisk
		fs = filesys.MakeFsOnDisk()

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

// ParseFileAsDockerCompose parses a Docker Compose file into a types.Config struct.
func ParseFileAsDockerComposeStrict(file SanitizedFile) (*dockerTypes.Config, error) {

	// Step 1: Parse YAML using Docker’s built-in loader (ensures correct types)
	parsedConfig, err := dockerLoader.ParseYAML(file.RawData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Docker Compose YAML: %v", err)
	}

	// Use Docker Compose's loader to parse the raw config into a types.Config
	configDetails := dockerTypes.ConfigDetails{
		ConfigFiles: []dockerTypes.ConfigFile{
			{
				Config: parsedConfig,
			},
		},
		Environment: map[string]string{}, // Optional: Add environment variables if needed
	}

	config, err := dockerLoader.Load(configDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to load Docker Compose config: %v", err)
	}

	return config, nil
}

type ParsedCompose struct {
	manifest string
}

// ParseFileAsDockerCompose parses a Docker Compose file into a types.Config struct.
func ParseFileAsDockerCompose(file SanitizedFile) (ParsedCompose, error) {

	manifest, err := kompose.Convert(file.RawData)

	// fmt.Println("manifest ", manifest)

	if err != nil {
		return ParsedCompose{}, fmt.Errorf("failed to load Docker Compose config: %v", err)
	}

	return ParsedCompose{
		manifest: manifest,
	}, nil
}
