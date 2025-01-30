package files

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
)

const MESHERY_DESIGN = "Design"
const KUBERNETES_MANIFEST = "KubernetesManifest"
const DOCKER_COMPOSE = "DockerCompose"
const KUSTOMIZATION = "Kustomization"
const HELM_CHART = "HelmChart"
const MESHERY_VIEW = "View"

type IdentifiedFile struct {
	Type string
	// pattern.PatternFile,etc
	ParsedFile interface{}
}

// func  IdentifyFileType(sanitizedFile:SanitizedFile) (string,error){

// }

func IdentifyFile(sanitizedFile SanitizedFile) (IdentifiedFile, error) {
	if parsed, err := ParseFileAsMesheryDesign(sanitizedFile); err == nil {
		fmt.Println("parsed")
		return IdentifiedFile{
			Type:       MESHERY_DESIGN,
			ParsedFile: parsed,
		}, nil
	}

	return IdentifiedFile{}, fmt.Errorf("Unsupported FileType")
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
