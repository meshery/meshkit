package registration

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	meshmodel "github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta1/model"
)

// packaingUnit is the representation of the atomic unit that can be registered into the capabilities registry
type packagingUnit struct {
	model         model.ModelDefinition
	components    []component.ComponentDefinition
	relationships []relationship.RelationshipDefinition
	_             []v1beta1.PolicyDefinition
}

type RegistrationHelper struct {
	regManager  *meshmodel.RegistryManager
	regErrStore RegistrationErrorStore
	svgBaseDir  string
}

func NewRegistrationHelper(svgBaseDir string, regm *meshmodel.RegistryManager, regErrStore RegistrationErrorStore) RegistrationHelper {
	return RegistrationHelper{svgBaseDir: svgBaseDir, regManager: regm, regErrStore: regErrStore}
}

/*
Register will accept a RegisterableEntity (dir, tar or oci for now).
*/
func (rh *RegistrationHelper) Register(entity RegisterableEntity) {
	// get the packaging units
	pu, err := entity.PkgUnit(rh.regErrStore)
	if err != nil {
		// given input is not a valid model, or could not walk the directory
		return
	}
	rh.register(pu)
}

/*
register will return an error if it is not able to register the `model`.
If there are errors when registering other entities, they are handled properly but does not stop the registration process.
*/
func (rh *RegistrationHelper) register(pkg packagingUnit) {
	// 1. Register the model
	model := pkg.model

	// Dont register anything else if registrant is not there
	if model.Registrant.Kind == "" {
		err := ErrMissingRegistrant(model.Name)
		rh.regErrStore.InsertEntityRegError(model.Registrant.Kind, "", entity.Model, model.Name, err)
		return
	}

	if model.Metadata != nil {
		svgComplete := ""
		if model.Metadata.SvgComplete != nil {
			svgComplete = *model.Metadata.SvgComplete
		}

		var svgCompletePath string

		//Write SVG for models
		model.Metadata.SvgColor, model.Metadata.SvgWhite, svgCompletePath = WriteAndReplaceSVGWithFileSystemPath(model.Metadata.SvgColor,
			model.Metadata.SvgWhite,
			svgComplete, rh.svgBaseDir,
			model.Name,
			model.Name,
		)
		if svgCompletePath != "" {
			model.Metadata.SvgComplete = &svgCompletePath
		}

	}
	_, _, err := rh.regManager.RegisterEntity(
		connection.Connection{Kind: model.Registrant.Kind},
		&model,
	)

	// If model cannot be registered, don't register anything else
	if err != nil {
		err = ErrRegisterEntity(err, string(model.Type()), model.DisplayName)
		rh.regErrStore.InsertEntityRegError(model.Registrant.Kind, "", entity.Model, model.Name, err)
		return
	}

	hostname := model.Registrant.Kind
	modelName := model.Name
	// 2. Register components
	for _, comp := range pkg.components {
		comp.Model = model

		if comp.Styles != nil {
			//Write SVG for components
			comp.Styles.SvgColor, comp.Styles.SvgWhite, comp.Styles.SvgComplete = WriteAndReplaceSVGWithFileSystemPath(
				comp.Styles.SvgColor,
				comp.Styles.SvgWhite,
				comp.Styles.SvgComplete,
				rh.svgBaseDir,
				comp.Model.Name,
				comp.Component.Kind,
			)
		}

		_, _, err := rh.regManager.RegisterEntity(
			connection.Connection{Kind: model.Registrant.Kind},
			&comp,
		)
		if err != nil {
			err = ErrRegisterEntity(err, string(comp.Type()), comp.DisplayName)
			rh.regErrStore.InsertEntityRegError(hostname, modelName, entity.ComponentDefinition, comp.DisplayName, err)
		}
	}

	// 3. Register relationships
	for _, rel := range pkg.relationships {
		rel.Model = model
		_, _, err := rh.regManager.RegisterEntity(connection.Connection{Kind: model.Registrant.Kind}, &rel)
		if err != nil {
			err = ErrRegisterEntity(err, string(rel.Type()), string(rel.Kind))
			rh.regErrStore.InsertEntityRegError(hostname, modelName, entity.RelationshipDefinition, rel.Id.String(), err)
		}
	}
}
