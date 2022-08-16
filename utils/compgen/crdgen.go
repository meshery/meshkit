package compgen

import (
// "github.com/layer5io/meshkit/utils"
// "github.com/sirupsen/logrus"
)

type crdComponentsGenerator struct {
	crds                []string
	extractorPathConfig CuePathConfig
	resourceMetadata    map[string]string
}

// crds should be yaml
// func (cg crdComponentsGenerator) generate() ([]Component, error) {
// 	components := make([]Component, 0)
// 	for _, crd := range cg.crds {
// 		// crd shoudl be YAML
// 		crdCue, err := utils.YamlToCue(crd)
// 		if err != nil {
// 			logrus.Warn(ErrCrdGenerate(err))
// 			continue
// 		}
// 		definition, err := getDefinition(crdCue, cg.extractorPathConfig, cg.resourceMetadata)
// 		if err != nil {
// 			logrus.Warn(ErrCrdGenerate(err))
// 			continue
// 		}
// 		schema, err := getSchema(crdCue, cg.extractorPathConfig)
// 		if err != nil {
// 			logrus.Warn(ErrCrdGenerate(err))
// 			continue
// 		}
// 		components = append(components, Component{Definition: definition, Schema: schema})
// 	}
// 	return components, nil
// }

// func NewCrdComponentGenerator(crds []string, pathConf CuePathConfig) ComponentsGenerator {
// 	return &crdComponentsGenerator{
// 		crds:                crds,
// 		extractorPathConfig: pathConf,
// 	}
// }
