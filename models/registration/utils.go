package registration

import (
	"fmt"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
)

// TODO: refactor this and use CUE
func getEntity(byt []byte) (et entity.Entity, _ error) {
	type schemaVersion struct {
		SchemaVersion string `json:"schemaVersion" yaml:"schemaVersion"`
	}
	var sv schemaVersion
	err := encoding.Unmarshal(byt, &sv)
	if err != nil || sv.SchemaVersion == "" {
		return nil, ErrGetEntity(fmt.Errorf("does not contain versionmeta"))
	}
	switch sv.SchemaVersion {
	case v1beta1.ComponentSchemaVersion:
		var compDef component.ComponentDefinition
		err := encoding.Unmarshal(byt, &compDef)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("invalid component definition: %s", err.Error()))
		}
		et = &compDef
	case v1beta1.ModelSchemaVersion:
		var model model.ModelDefinition
		err := encoding.Unmarshal(byt, &model)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("invalid model definition: %s", err.Error()))
		}
		et = &model
	case v1alpha3.RelationshipSchemaVersion:
		var rel relationship.RelationshipDefinition
		err := encoding.Unmarshal(byt, &rel)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("invalid relationship definition: %s", err.Error()))
		}
		et = &rel
	default:
		return nil, ErrGetEntity(fmt.Errorf("not a valid component definition, model definition, or relationship definition"))
	}
	return et, nil
}
