package component

import (
	"encoding/json"
	"strings"

	"cuelang.org/go/cue"
	"github.com/layer5io/meshkit/models/oam/core/v1alpha1"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/manifests"
)

func getDefinition(crd cue.Value, pathConf CuePathConfig, metadata map[string]string) (string, error) {
	var def v1alpha1.WorkloadDefinition

	resourceId, err := extractValueFromPath(crd, pathConf.IdentifierPath)
	if err != nil {
		return "", ErrGetDefinition(err)
	}
	// apiVersion, err := extractValueFromPath(crd, pathConf.VersionPath)
	// if err != nil {
	// 	return "", ErrGetDefinition(err)
	// }
	// apiGroup, err := extractValueFromPath(crd, pathConf.GroupPath)
	// if err != nil {
	// 	return "", ErrGetDefinition(err)
	// }

	definitionRef := strings.ToLower(resourceId) + ".meshery.layer5.io"
	def.Spec.DefinitionRef.Name = definitionRef
	def.ObjectMeta.Name = resourceId
	def.APIVersion = "core.oam.dev/v1alpha1"
	def.Kind = "WorkloadDefinition"
	def.Spec.Metadata = map[string]string{
		"@type": "pattern.meshery.io/core",
	}
	// append metadata
	for k, v := range metadata {
		def.Spec.Metadata[k] = v
	}
	out, err := json.MarshalIndent(def, "", " ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

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

	resourceId, err := extractValueFromPath(parsedCrd, pathConf.IdentifierPath)
	if err != nil {
		return "", ErrGetSchema(err)
	}

	(schema)["title"] = manifests.FormatToReadableString(resourceId)
	var output []byte
	output, err = json.MarshalIndent(schema, "", " ")
	if err != nil {
		return "", ErrGetSchema(err)
	}
	return string(output), nil
}

func extractValueFromPath(crd cue.Value, pathConf string) (string, error) {
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
