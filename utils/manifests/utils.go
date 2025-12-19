package manifests

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/models/oam/core/v1alpha1"
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
	def.Name = resourceId
	def.APIVersion = "core.oam.dev/v1alpha1"
	def.Kind = "WorkloadDefinition"
	k8sAPIVersion := apiVersion
	if apiGroup != "" {
		k8sAPIVersion = apiGroup + "/" + k8sAPIVersion
	}
	switch resource {
	case SERVICE_MESH:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/mesh/workload",
			"meshVersion":   cfg.MeshVersion,
			"meshName":      cfg.Name,
			"k8sAPIVersion": k8sAPIVersion,
			"k8sKind":       resourceId,
		}
		def.Spec.DefinitionRef.Name = strings.ToLower(resourceId)
		if cfg.Type != "" {
			def.Name += "." + cfg.Type
			def.Spec.DefinitionRef.Name += "." + cfg.Type
		}
		def.Spec.DefinitionRef.Name += ".meshery.layer5.io"
	case K8s:
		def.Spec.Metadata = map[string]string{
			"@type":         "pattern.meshery.io/k8s",
			"k8sAPIVersion": k8sAPIVersion,
			"k8sKind":       resourceId,
			"version":       cfg.K8sVersion,
		}
		def.Name += ".K8s"
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

// This helps in formating the leftover fields using a pre-defined dictionary
// If dictionary returns true then any other formatting should be skipped
// The key in the dictionary contains completely unedited fields that are expected in certain input files
// The value is then the in-place replacement for those specific words
func useDictionary(input string, invert bool) (string, bool) {
	dict := map[string]string{
		"MeshSync":             "MeshSync",
		"additionalProperties": "additionalProperties",
		"caBundle":             "CA Bundle",
		"mtls":                 "mTLS",
		"mTLS":                 "mTLS",
	}
	for comp, val := range dict {
		if invert {
			if val == input {
				fmt.Println("inverted ", val, " to ", comp)
				return comp, true
			}
		} else {
			if comp == input {
				return val, true
			}
		}
	}
	return input, false
}

// While going from Capital letter to small, insert a whitespace before the capital letter.
// While going from small letter to capital, insert a whitespace after the small letter
// The above is a general rule and further "exceptions" are used.
func FormatToReadableString(input string) string {
	if len(input) == 0 {
		return ""
	}
	input, found := useDictionary(input, false)
	if found {
		return input
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
	return strings.Join(strings.Fields(strings.TrimSpace(finalWord+input[len(input)-1:])), " ")
}

func DeFormatReadableString(input string) string {
	output, found := useDictionary(input, true)
	if found {
		return output
	}
	return strings.ReplaceAll(output, " ", "")
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

// TODO: Refactor to use interface{} as an argument while doing type conversion recursively instead of assuming the input to always be a marshaled map[string]interface{}
func (ro *ResolveOpenApiRefs) ResolveReferences(manifest []byte, definitions cue.Value, cache map[string][]byte) ([]byte, error) {
	setIsJsonFalse := func() {
		ro.isInsideJsonSchemaProps = false
	}
	if cache == nil {
		cache = make(map[string][]byte)
	}
	var val map[string]interface{}
	err := encoding.Unmarshal(manifest, &val)
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
				marVal, errJson := encoding.Marshal(val)
				if errJson != nil {
					return manifest, nil
				}
				return marVal, nil
			}
		}
	}
	for k, v := range val {
		if v, ok := v.([]interface{}); ok {
			newval := make([]interface{}, 0)
			for _, v0 := range v {
				if _, ok := v0.(map[string]interface{}); !ok {
					newval = append(newval, v0)
					continue
				}
				byt, _ := encoding.Marshal(v0)
				byt, err = ro.ResolveReferences(byt, definitions, cache)
				if err != nil {
					return nil, err
				}
				var newvalmap map[string]interface{}
				_ = encoding.Unmarshal(byt, &newvalmap)
				newval = append(newval, newvalmap)
			}
			val[k] = newval
		}
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
				var def []byte
				if cache[path] == nil {
					refVal := definitions.LookupPath(cue.ParsePath(fmt.Sprintf(`"%v"`, path)))
					if refVal.Err() != nil {
						return nil, refVal.Err()
					}
					marshalledVal, errJson := refVal.MarshalJSON()
					if errJson != nil {
						return nil, err
					}
					def, err = ro.ResolveReferences(marshalledVal, definitions, cache)
					if err != nil {
						return nil, err
					}
					cache[path] = def
				} else {
					def = cache[path]
				}
				if def != nil {
					err = replaceRefWithVal(def, val, k)
					if err != nil {
						return def, nil
					}
				}
				return def, nil
			}
		}
		if reflect.ValueOf(v).Kind() == reflect.Map {
			var marVal []byte
			var def []byte
			marVal, err = encoding.Marshal(v)
			if err != nil {
				return nil, err
			}
			def, err = ro.ResolveReferences(marVal, definitions, cache)
			if err != nil {
				return nil, err
			}
			if def != nil {
				err = replaceRefWithVal(def, val, k)
				if err != nil {
					return def, nil
				}
			}
		}
	}
	res, err := encoding.Marshal(val)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func replaceRefWithVal(def []byte, val map[string]interface{}, k string) error {
	var defVal map[string]interface{}
	err := encoding.Unmarshal([]byte(def), &defVal)
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
