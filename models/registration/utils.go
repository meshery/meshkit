package registration

import (
	"encoding/json"
	"fmt"

	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
	"gopkg.in/yaml.v2"
)

func unmarshal(byt []byte, out interface{}) error {
	err := json.Unmarshal(byt, out)
	if err != nil {
		err = yaml.Unmarshal(byt, out)
		if err != nil {
			return fmt.Errorf("Not a valid YAML or JSON")
		}
	}
	return nil
}

// TODO: refactor this and use CUE
func getEntity(byt []byte) (et entity.Entity, _ error) {
	type schemaVersion struct {
		SchemaVersion string `json:"schemaVersion" yaml:"schemaVersion"`
	}
	var sv schemaVersion
	err := unmarshal(byt, &sv)
	if err != nil || sv.SchemaVersion == "" {
		return nil, ErrGetEntity(fmt.Errorf("Does not contain versionmeta"))
	}
	switch sv.SchemaVersion {
	case v1beta1.ComponentSchemaVersion:
		var compDef component.ComponentDefinition
		err := unmarshal(byt, &compDef)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid component definition: %s", err.Error()))
		}
		et = &compDef
	case v1beta1.ModelSchemaVersion:
		var model model.ModelDefinition
		err := unmarshal(byt, &model)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid model definition: %s", err.Error()))
		}
		et = &model
	case v1alpha3.RelationshipSchemaVersion:
		var rel relationship.RelationshipDefinition
		err := unmarshal(byt, &rel)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid relationship definition: %s", err.Error()))
		}
		et = &rel
	default:
		return nil, ErrGetEntity(fmt.Errorf("Not a valid component definition, model definition, or relationship definition"))
	}
	return et, nil
}
