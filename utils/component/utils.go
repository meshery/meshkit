package component

import (
	"encoding/json"

	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/manifests"
)

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
