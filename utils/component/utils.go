package component

import (
	"encoding/json"

	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshkit/utils/manifests"
	"gopkg.in/yaml.v2"
)

// Remove the fields which is either not required by end user (like status) or is prefilled by system (like apiVersion, kind and metadata)
var fieldsToDelete = [4]string{"apiVersion", "kind", "status", "metadata"}

// extracts the JSONSCHEMA of the CRD and outputs the json encoded string of the schema
func getSchema(parsedCrd cue.Value, pathConf CuePathConfig) (string, error) {
	schema := map[string]interface{}{}
	specCueVal, err := utils.Lookup(parsedCrd, pathConf.SpecPath)
	if err != nil {
		return "", err
	}
	marshalledJson, err := specCueVal.MarshalJSON()
	if err != nil {
		return "", ErrGetSchema(err)
	}
	err = json.Unmarshal(marshalledJson, &schema)
	if err != nil {
		return "", ErrGetSchema(err)
	}
	resourceId, err := extractCueValueFromPath(parsedCrd, pathConf.IdentifierPath)
	if err != nil {
		return "", ErrGetSchema(err)
	}

	updatedProps, err := UpdateProperties(specCueVal, cue.ParsePath(pathConf.PropertiesPath), resourceId)

	if err != nil {
		return "", err
	}

	schema = updatedProps
	DeleteFields(schema)

	(schema)["title"] = manifests.FormatToReadableString(resourceId)
	var output []byte
	output, err = json.MarshalIndent(schema, "", " ")
	if err != nil {
		return "", ErrGetSchema(err)
	}
	return string(output), nil
}

func extractCueValueFromPath(crd cue.Value, pathConf string) (string, error) {
	cueRes, err := utils.Lookup(crd, pathConf)
	if err != nil {
		return "", err
	}
	res, err := cueRes.String()
	if err != nil {
		return "", err
	}
	return res, nil
}

// function to remove fields that are not required or prefilled
func DeleteFields(m map[string]interface{}) {
	key := "properties"
	if m[key] == nil {
		return
	}
	if prop, ok := m[key].(map[string]interface{}); ok && prop != nil {
		for _, f := range fieldsToDelete {
			delete(prop, f)
		}
		m[key] = prop
	}
}

func FilterCRDs(manifests [][]byte) ([]string, []error) {
	var errs []error
	var filteredManifests []string
	for _, m := range manifests {

		var crd map[string]interface{}
		err := yaml.Unmarshal(m, &crd)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		isCrd := kubernetes.IsCRD(crd)
		if !isCrd {
			continue
		}
		filteredManifests = append(filteredManifests, string(m))
	}
	return filteredManifests, errs
}
