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

const (
	JsonSchemaPropsRef = "JSONSchemaProps"
)

var templateExpression *regexp.Regexp

func getDefinitions(parsedCrd cue.Value, resource int, cfg Config, ctx context.Context) (string, error) {
	var def v1alpha1.WorkloadDefinition
	// get the resource identifier
	idCueVal, _ := cfg.CrdFilter.IdentifierExtractor(parsedCrd)
	resourceId, err := idCueVal.String()
	if err != nil {
		return "", ErrGetResourceIdentifier(err)
	}
	definitionRef := strings.ToLower(resourceId) + ".meshery.layer5.io"
	apiVersionCueVal, _ := cfg.CrdFilter.VersionExtractor(parsedCrd)
	apiVersion, err := apiVersionCueVal.String()
	if err != nil {
		return "", ErrGetAPIVersion(err)
	}
	apiGroupCueVal, _ := cfg.CrdFilter.GroupExtractor(parsedCrd)
	apiGroup, err := apiGroupCueVal.String()
	if err != nil {
		return "", ErrGetAPIGroup(err)
	}
	//getting defintions for different native resources
	def.Spec.DefinitionRef.Name = definitionRef
	def.ObjectMeta.Name = resourceId
	def.APIVersion = "core.oam.dev/v1alpha1"
	def.Kind = "WorkloadDefinition"
	switch resource {
	case SERVICE_MESH:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/mesh/workload",
			"meshVersion":   cfg.MeshVersion,
			"meshName":      cfg.Name,
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       resourceId,
		}
		def.Spec.DefinitionRef.Name = strings.ToLower(resourceId)
		if cfg.Type != "" {
			def.ObjectMeta.Name += "." + cfg.Type
			def.Spec.DefinitionRef.Name += "." + cfg.Type
		}
		def.Spec.DefinitionRef.Name += ".meshery.layer5.io"
	case K8s:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/k8s",
			"k8sAPIVersion": apiGroup + "/" + apiVersion,
			"k8sKind":       resourceId,
			"version":       cfg.K8sVersion,
		}
		def.ObjectMeta.Name += ".K8s"
		def.Spec.DefinitionRef.Name = strings.ToLower(resourceId) + ".k8s.meshery.layer5.io"
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
	specCueVal, _ := cfg.CrdFilter.SpecExtractor(parsedCrd)
	marshalledJson, err := specCueVal.MarshalJSON()
	if err != nil {
		return "", ErrGetSchemas(err)
	}
	err = json.Unmarshal(marshalledJson, &schema)
	if err != nil {
		return "", ErrGetSchemas(err)
	}
	idCueVal, _ := cfg.CrdFilter.IdentifierExtractor(parsedCrd)
	resourceId, err := idCueVal.String()
	if err != nil {
		return "", ErrGetResourceIdentifier(err)
	}
	(schema)["title"] = FormatToReadableString(resourceId)
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

// removeMetadataFromCRD is used because in few cases (like linkerd), helm templating might be used there which makes the yaml invalid.
// As those templates are useless for component creatin, we can replace them with "meshery" to make the YAML valid
func RemoveHelmTemplatingFromCRD(crdyaml *string) {
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

// cleanup
func deleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	return nil
}

// This helps in formating the leftover fields using a pre-defined dictionary
func useDictionary(input string) string {
	dict := map[string]string{ // includes Whitelist words
		"Mesh Sync":             "MeshSync",
		"additional Properties": "additionalProperties", //This field is used by RJSF and should not include white space
		"ca Bundle" : "CA Bundle",
	}
	for comp := range dict {
		if comp == input {
			return dict[comp]
		}
	}
	return input
}

// While going from Capital letter to small, insert a whitespace before the capital letter.
// While going from small letter to capital, insert a whitespace after the small letter
// The above is a general rule and further "exceptions" are used.
func FormatToReadableString(input string) string {
	if len(input) == 0 {
		return ""
	}
	finalWord := string(input[0])
	for i := range input {
		if i == 0 {
			continue
		}
		if i == len(input)-1 {
			break
		}
		switch actionToPerform(i-1, i, i+1, input) {
		case dontaddspace:
			finalWord += string(input[i])
		case addleadingspace:
			finalWord += string(input[i]) + " "
		case addtrailingspace:
			finalWord += " " + string(input[i])
		}
	}
	return useDictionary(strings.Join(strings.Fields(strings.TrimSpace(finalWord+input[len(input)-1:])), " "))
}

const (
	dontaddspace     = 0
	addtrailingspace = -1
	addleadingspace  = 1
)

func isBig(ch byte) bool {
	return int(ch) >= 65 && int(ch) <= 90
}
func isSmall(ch byte) bool {
	return int(ch) >= 97 && int(ch) <= 122
}

func actionToPerform(prev int, curr int, next int, input string) int {
	if isException(prev, curr, next, input) {
		return dontaddspace
	}
	// previsSmall := isBig(input[prev])
	previsBig := isBig(input[prev])
	currisSmall := isSmall(input[curr])
	currisBig := isBig(input[curr])
	nextisSmall := isSmall(input[next])
	nextisBig := isBig(input[next])
	if currisSmall && nextisBig {
		return addleadingspace
	}
	if currisBig && previsBig && nextisSmall {
		return addtrailingspace
	}

	return dontaddspace
}

func init() {
	templateExpression = regexp.MustCompile(`{{.+}}`)
}

// Change this code to add more exception logic to bypass addition of space
func isException(prev int, curr int, next int, input string) (isException bool) {
	if next != len(input)-1 && isBig(input[curr]) && isBig(input[prev]) && isSmall(input[next]) && isBig(input[next+1]) { //For alternating text, like ClusterIPsRoute => Cluster Ips Route
		isException = true
	}
	if next == len(input)-1 && isSmall(input[next]) {
		isException = true
	}
	if isBig(input[curr]) && isSmall(input[next]) && next != len(input)-1 && isBig(input[next+1]) {
		isException = true
	}
	return
}

type ResolveOpenApiRefs struct {
	// this is used to track whether we have a JsonSchemaProp inside a JsonSchemaProp
	// which on resolving would cause infinite loops
	isInsideJsonSchemaProps bool
}

// we are manually dereferencing this because there are no other alternatives
// other alternatives to lookout for in the future are
//   1. cue's jsonschema encoding package
//    currently, it does not support resolving refs from external world
//   2. cue's openapi encoding package (currently it only supports openapiv3)

//	for resolving refs in kubernetes openapiv2 jsonschema
//
// definitions - parsed CUE value of the 'definitions' in openapiv2
// manifest - jsonschema manifest to resolve refs for
func (ro *ResolveOpenApiRefs) ResolveReferences(manifest []byte, definitions cue.Value) ([]byte, error) {
	setIsJsonFalse := func() {
		ro.isInsideJsonSchemaProps = false
	}
	var val map[string]interface{}
	err := json.Unmarshal(manifest, &val)
	if err != nil {
		return nil, err
	}
	if val["$ref"] != nil {
		if ref, ok := val["$ref"].(string); ok {
			splitRef := strings.Split(ref, ".")
			ref := splitRef[len(splitRef)-1]
			// if we have a JsonSchemaProp inside a JsonSchema Prop
			if ro.isInsideJsonSchemaProps && (ref == JsonSchemaPropsRef) {
				// hack so that the UI doesn't crash
				val["$ref"] = "string"
				marVal, err := json.Marshal(val)
				if err != nil {
					return manifest, nil
				}
				return marVal, nil
			}
		}
	}
	for k, v := range val {
		if k == "$ref" {
			if v, ok := v.(string); ok {
				splitRef := strings.Split(v, ".")
				ref := splitRef[len(splitRef)-1]
				if ref == JsonSchemaPropsRef {
					ro.isInsideJsonSchemaProps = true
					defer setIsJsonFalse()
				}
				splt := strings.Split(v, "/")
				path := splt[len(splt)-1]
				refVal := definitions.LookupPath(cue.ParsePath(fmt.Sprintf(`"%v"`, path)))
				if refVal.Err() != nil {
					return nil, refVal.Err()
				}
				marshalledVal, err := refVal.MarshalJSON()
				if err != nil {
					return nil, err
				}
				def, err := ro.ResolveReferences(marshalledVal, definitions)
				if err != nil {
					return nil, err
				}
				if def != nil {
					err := replaceRefWithVal(def, val, k)
					if err != nil {
						return def, nil
					}
				}
				return def, nil
			}
		}
		if reflect.ValueOf(v).Kind() == reflect.Map {
			marVal, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			def, err := ro.ResolveReferences(marVal, definitions)
			if err != nil {
				return nil, err
			}
			if def != nil {
				err := replaceRefWithVal(def, val, k)
				if err != nil {
					return def, nil
				}
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

type ExtractorPaths struct {
	NamePath    string
	GroupPath   string
	VersionPath string
	SpecPath    string
	IdPath      string
}

func NewCueCrdFilter(ep ExtractorPaths, isJson bool) CueCrdFilter {
	return CueCrdFilter{
		IsJson: isJson,
		IdentifierExtractor: func(rootCRDCueVal cue.Value) (cue.Value, error) {
			res := rootCRDCueVal.LookupPath(cue.ParsePath(ep.IdPath))
			if !res.Exists() {
				return res, fmt.Errorf("Could not find the value")
			}
			return res.Value(), nil
		},
		NameExtractor: func(rootCRDCueVal cue.Value) (cue.Value, error) {
			res := rootCRDCueVal.LookupPath(cue.ParsePath(ep.NamePath))
			if !res.Exists() {
				return res, fmt.Errorf("Could not find the value")
			}
			return res.Value(), nil
		},
		VersionExtractor: func(rootCRDCueVal cue.Value) (cue.Value, error) {
			res := rootCRDCueVal.LookupPath(cue.ParsePath(ep.VersionPath))
			if !res.Exists() {
				return res, fmt.Errorf("Could not find the value")
			}
			return res.Value(), nil
		},
		GroupExtractor: func(rootCRDCueVal cue.Value) (cue.Value, error) {
			res := rootCRDCueVal.LookupPath(cue.ParsePath(ep.GroupPath))
			if !res.Exists() {
				return res, fmt.Errorf("Could not find the value")
			}
			return res.Value(), nil
		},
		SpecExtractor: func(rootCRDCueVal cue.Value) (cue.Value, error) {
			res := rootCRDCueVal.LookupPath(cue.ParsePath(ep.SpecPath))
			if !res.Exists() {
				return res, fmt.Errorf("Could not find the value")
			}
			return res.Value(), nil
		},
	}
}
