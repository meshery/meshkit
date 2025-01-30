package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
	k8sYaml "k8s.io/apimachinery/pkg/util/yaml"

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
	// pattern.PatternFile,helm_loader,etc
	ParsedFile interface{}
}

func IdentifyFile(sanitizedFile SanitizedFile) (IdentifiedFile, error) {
	var err error
	var parsed interface{}

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
