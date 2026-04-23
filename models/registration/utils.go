package registration

import (
	"fmt"

	"github.com/meshery/meshkit/encoding"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/schema"
	"github.com/meshery/schemas/models/v1alpha3"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/model"
	"github.com/meshery/schemas/models/v1beta3"
	"github.com/meshery/schemas/models/v1beta3/component"
)

// TODO: refactor this and use CUE
func getEntity(byt []byte) (et entity.Entity, _ error) {
	type schemaVersion struct {
		SchemaVersion string `json:"schemaVersion" yaml:"schemaVersion"`
	}
	var sv schemaVersion
	err := encoding.Unmarshal(byt, &sv)
	if err != nil || sv.SchemaVersion == "" {
		return nil, ErrGetEntity(fmt.Errorf("Does not contain versionmeta"))
	}
	switch sv.SchemaVersion {
	// Accept both the legacy v1beta1 and the canonical-casing v1beta3 schema
	// version strings for components and models. v1beta3 lifted the component
	// and model wire format to camelCase (see v1beta3/const.go); the legacy
	// v1beta1 strings remain accepted for backward compatibility with
	// existing on-disk definitions that have not been regenerated yet.
	case v1beta1.ComponentSchemaVersion, v1beta3.ComponentSchemaVersion:
		var compDef component.ComponentDefinition
		err := encoding.Unmarshal(byt, &compDef)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid component definition: %s", err.Error()))
		}
		et = &compDef
	case v1beta1.ModelSchemaVersion, v1beta3.ModelSchemaVersion:
		var model model.ModelDefinition
		err := encoding.Unmarshal(byt, &model)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid model definition: %s", err.Error()))
		}
		et = &model
	case schema.RelationshipSchemaVersionV1Beta2, v1alpha3.RelationshipSchemaVersion:
		var rel relationship.RelationshipDefinition
		err := encoding.Unmarshal(byt, &rel)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid relationship definition: %s", err.Error()))
		}
		et = &rel
	default:
		return nil, ErrGetEntity(fmt.Errorf("Not a valid component definition, model definition, or relationship definition"))
	}
	return et, nil
}
