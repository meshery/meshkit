package manifests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
)

var templateExpression *regexp.Regexp

func getDefinitions(parsedCrd cue.Value, resource int, cfg Config, ctx context.Context) (string, error) {

	var def v1alpha1.WorkloadDefinition
	idCueVal, err := cfg.CrdFilter.IdentifierExtractor(parsedCrd)
	id, err := idCueVal.String()
	if err != nil {
		return "", ErrGetAPIVersion(err) // have to change it to Id error
	}
	definitionRef := strings.ToLower(id) + ".meshery.layer5.io"
	apiVersionCueVal, err := cfg.CrdFilter.VersionExtractor(parsedCrd)
	apiVersion, err := apiVersionCueVal.String()
	if err != nil {
		return "", ErrGetAPIVersion(err)
	}

	apiGroupCueVal, err := cfg.CrdFilter.GroupExtractor(parsedCrd)
	apiGroup, err := apiGroupCueVal.String()
	if err != nil {
		return "", ErrGetAPIGroup(err)
	}

	//getting defintions for different native resources
	def.Spec.DefinitionRef.Name = definitionRef
	def.ObjectMeta.Name = id
	def.APIVersion = "core.oam.dev/v1alpha1"
	def.Kind = "WorkloadDefinition"
	switch resource {
	case SERVICE_MESH:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/mesh/workload",
			"meshVersion":   cfg.MeshVersion,
			"meshName":      cfg.Name,
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       id,
		}
		def.Spec.DefinitionRef.Name = strings.ToLower(id)
		if cfg.Type != "" {
			def.ObjectMeta.Name += "." + cfg.Type
			def.Spec.DefinitionRef.Name += "." + cfg.Type
		}
		def.Spec.DefinitionRef.Name += ".meshery.layer5.io"
	case K8s:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/k8s",
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       id,
			"version":       cfg.K8sVersion,
		}
		def.ObjectMeta.Name += ".K8s"
		def.Spec.DefinitionRef.Name = strings.ToLower(id) + ".k8s.meshery.layer5.io"
	case MESHERY:
		def.Spec.Metadata = map[string]string{
			"@type": "pattern.meshery.io/core",
		}
	}
	out, err := json.MarshalIndent(def, "", " ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getSchema(parsedCrd cue.Value, cfg Config, ctx context.Context) (string, error) {
	schema := map[string]interface{}{}

	specCueVal, err := cfg.CrdFilter.SpecExtractor(parsedCrd)
	marshalledJson, err := specCueVal.MarshalJSON()
	err = json.Unmarshal(marshalledJson, &schema)

	if err != nil {
		return "", err
	}

	idCueVal, err := cfg.CrdFilter.IdentifierExtractor(parsedCrd)

	id, err := idCueVal.String()
	if err != nil {
		return "", ErrGetSchemas(err) // have to change it to Id error
	}

	(schema)["title"] = FormatToReadableString(id)
	var output []byte
	output, err = json.MarshalIndent(schema, "", " ")
	if err != nil {
		return "", err
	}
	return string(output), nil
}
func populateTempyaml(yaml string, path string) error {
	var _, err = os.Stat(path)
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	//delete any previous contents from the file
	if err := os.Truncate(path, 0); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(yaml)
	if err != nil {
		return err
	}
	err = file.Sync()
	if err != nil {
		return err
	}

	return nil
}

//removeMetadataFromCRD is used because in few cases (like linkerd), helm templating might be used there which makes the yaml invalid.
//As those templates are useless for component creatin, we can replace them with "meshery" to make the YAML valid
func removeHelmTemplatingFromCRD(crdyaml *string) {
	y := strings.Split(*crdyaml, "\n---\n")
	var yamlArr []string
	for _, y0 := range y {
		if y0 == "" {
			continue
		}
		y0 = templateExpression.ReplaceAllString(y0, "meshery")
		yamlArr = append(yamlArr, string(y0))
	}
	*crdyaml = strings.Join(yamlArr, "\n---\n")
}

func getCrdnames(s string) []string {
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", "")
	crds := strings.Split(s, "\n")
	if len(crds) <= 2 {
		return []string{}
	}
	return crds[1 : len(crds)-2] // first and last characters are "[" and "]" respectively
}

func filterYaml(ctx context.Context, yamlPath string, filter []string, binPath string, inputFormat string) error {
	var (
		out bytes.Buffer
		er  bytes.Buffer
	)
	filter = append(filter, "-o", "yaml")
	getCrdsCmdArgs := append([]string{"--location", yamlPath, "-t", inputFormat, "--filter"}, filter...)
	cmd := exec.CommandContext(ctx, binPath, getCrdsCmdArgs...)
	//emptying buffers
	out.Reset()
	er.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &er
	err := cmd.Run()
	if err != nil {
		return err
	}
	path := filepath.Join(os.TempDir(), "/test.yaml")
	err = populateTempyaml(out.String(), path)
	if err != nil {
		return ErrPopulatingYaml(err)
	}
	return nil
}

//cleanup
func deleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

//While going from Capital letter to small, insert a whitespace before the capital letter.
//While going from small letter to capital, insert a whitespace after the small letter
func FormatToReadableString(input string) string {
	if len(input) == 0 {
		return ""
	}
	finalWord := ""
	for i := range input {
		if i == len(input)-1 {
			break
		}
		switch switchedCasing(input[i], input[i+1]) {
		case samegroup:
			finalWord += string(input[i])
		case smallToBig:
			finalWord += string(input[i]) + " "
		case bigToSmall:
			finalWord += " " + string(input[i])
		}
	}
	return strings.Join(strings.Fields(strings.TrimSpace(finalWord+input[len(input)-1:])), " ")
}

const (
	samegroup  = 0
	smallToBig = 1
	bigToSmall = -1
)

//switchedCasting returns 0 if a and b are both small, or both Capital letter.
//returns 1 when a is small, but b is capital
//returns -1 otherwise
func switchedCasing(a byte, b byte) int {
	aisSmall := int(a) >= 97 && int(a) <= 122
	bisSmall := int(b) >= 97 && int(b) <= 122
	if aisSmall && !bisSmall {
		return smallToBig
	}
	if bisSmall && !aisSmall {
		return bigToSmall
	}
	return samegroup
}

func init() {
	templateExpression = regexp.MustCompile(`{{.+}}`)
}

// we are manually dereferencing this because there are no other alternatives
// other alternatives to lookout for
//   1. cue's jsonschema encoding package
//   2. cue's openapi encoding package (currently it only supports openapiv3)
func ResolveReferences(manifest []byte, definitions cue.Value) ([]byte, error) {
	var val map[string]interface{}
	err := json.Unmarshal(manifest, &val)
	if err != nil {
		return nil, err
	}

	// to get rid of cycles
	if val["$ref"] != nil {
		if reflect.ValueOf(val["$ref"]).Kind() != reflect.String {
			return manifest, nil
		}
	}

	for k, v := range val {
		if k == "$ref" {
			splt := strings.Split(v.(string), "/")
			path := splt[len(splt)-1]

			refVal := definitions.LookupPath(cue.ParsePath(fmt.Sprintf(`"%v"`, path)))

			if refVal.Err() != nil {
				return nil, refVal.Err()
			}
			marshalledVal, err := refVal.MarshalJSON()
			if err != nil {
				return nil, err
			}
			def, err := ResolveReferences(marshalledVal, definitions)
			if err != nil {
				return nil, err
			}
			if def != nil {
				replaceRefWithVal(def, val, k)
			}
			return def, nil
		}
		if reflect.ValueOf(v).Kind() == reflect.Map {
			marVal, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			def, err := ResolveReferences(marVal, definitions)
			if err != nil {
				return nil, err
			}
			if def != nil {
				replaceRefWithVal(def, val, k)
			}
		}
	}

	res, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func replaceRefWithVal(def []byte, val map[string]interface{}, k string) error {
	var defVal map[string]interface{}
	err := json.Unmarshal([]byte(def), &defVal)
	if err != nil {
		return err
	}
	val[k] = defVal
	return nil
}
