package converter

import (
	"bytes"

	"github.com/layer5io/meshkit/models/patterns"
	"github.com/layer5io/meshkit/utils"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/pattern"
	"gopkg.in/yaml.v3"
)

type K8sConverter struct{}

func (k *K8sConverter) Convert(patternFile string) (string, error) {
	pattern, err := patterns.GetPatternFormat(patternFile)
	if err != nil {
		return "", err
	}
	return NewK8sManifestsFromPatternfile(pattern)
}

func NewK8sManifestsFromPatternfile(patternFile *pattern.PatternFile) (string, error) {

	buf := bytes.NewBufferString("")

	enc := yaml.NewEncoder(buf)
	for _, comp := range patternFile.Components {
		err := enc.Encode(CreateK8sResourceStructure(comp))
		if err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func CreateK8sResourceStructure(comp *component.ComponentDefinition) map[string]interface{} {
	annotations := map[string]interface{}{}
	labels := map[string]interface{}{}

	_confMetadata, ok := comp.Configuration["metadata"]
	if ok {
		confMetadata, err := utils.Cast[map[string]interface{}](_confMetadata)
		if err == nil {

			_annotations, ok := confMetadata["annotations"]
			if ok {
				annotations, _ = utils.Cast[map[string]interface{}](_annotations)
			}

			_label, ok := confMetadata["labels"]

			if ok {
				labels, _ = utils.Cast[map[string]interface{}](_label)
			}
		}
	}

	component := map[string]interface{}{
		"apiVersion": comp.Component.Version,
		"kind":       comp.Component.Kind,
		"metadata": map[string]interface{}{
			"name":        comp.DisplayName,
			"annotations": annotations,
			"labels":      labels,
		},
	}

	for k, v := range comp.Configuration {
		if k == "apiVersion" || k == "kind" || k == "metadata" {
			continue
		}

		component[k] = v
	}
	return component
}
