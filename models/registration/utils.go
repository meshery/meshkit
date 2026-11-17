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
	connectionv1beta3 "github.com/meshery/schemas/models/v1beta3/connection"
	relationshipv1beta3 "github.com/meshery/schemas/models/v1beta3/relationship"
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
	// Accept every schema version string a component definition is written
	// with: the legacy v1beta1 string, components.meshery.io/v1beta2 (what
	// v1beta3.ComponentSchemaVersion names: the components nested in a
	// v1beta3 design), and components.meshery.io/v1beta3, the default of the
	// standalone v1beta3 component construct and of its templates. All three
	// decode into the v1beta3 component definition. Rejecting the v1beta3
	// string made every model scaffolded by `mesheryctl model init`
	// unimportable ("No valid component or relationship found").
	case v1beta1.ComponentSchemaVersion, v1beta3.ComponentSchemaVersion, schema.ComponentSchemaVersionV1Beta3:
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
	// The standalone v1beta3 relationship construct has its own struct. The
	// registry stores v1alpha3 relationships, so decode into the v1beta3 type
	// first and convert losslessly (an error is returned if any populated
	// field cannot be carried over, instead of silently dropping it).
	case schema.RelationshipSchemaVersionV1Beta3:
		var relV3 relationshipv1beta3.RelationshipDefinition
		if err := encoding.Unmarshal(byt, &relV3); err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid relationship definition: %s", err.Error()))
		}
		rel, err := convertRelationshipV1Beta3(&relV3)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid relationship definition: %s", err.Error()))
		}
		et = rel
	case v1beta3.ConnectionSchemaVersion:
		var connDef connectionv1beta3.ConnectionDefinition
		err := encoding.Unmarshal(byt, &connDef)
		if err != nil {
			return nil, ErrGetEntity(fmt.Errorf("Invalid connection definition: %s", err.Error()))
		}
		et = &connDef
	default:
		return nil, ErrGetEntity(fmt.Errorf("Not a valid component definition, model definition, relationship definition, or connection definition"))
	}
	return et, nil
}
