package registration

import (
	"github.com/meshery/meshkit/models/meshmodel/core/v1beta1"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	meshmodel "github.com/meshery/meshkit/models/meshmodel/registry"
	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/connection"
	"github.com/meshery/schemas/models/v1beta1/model"
)

// PackagingUnit is the representation of the atomic unit that can be registered into the capabilities registry
type PackagingUnit struct {
	Model         model.ModelDefinition
	Components    []component.ComponentDefinition
	Relationships []relationship.RelationshipDefinition
	Connections   []v1beta1.ConnectionDefinition
	_             []v1beta1.PolicyDefinition
}

type RegistrationHelper struct {
	regManager  *meshmodel.RegistryManager
	regErrStore RegistrationErrorStore
	svgBaseDir  string
	PkgUnits    []PackagingUnit // Store successfully registered packagingUnits
}

func NewRegistrationHelper(svgBaseDir string, regm *meshmodel.RegistryManager, regErrStore RegistrationErrorStore) RegistrationHelper {
	return RegistrationHelper{svgBaseDir: svgBaseDir, regManager: regm, regErrStore: regErrStore, PkgUnits: []PackagingUnit{}}
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
func (rh *RegistrationHelper) register(pkg PackagingUnit) {
	if len(pkg.Components) == 0 && len(pkg.Relationships) == 0 && len(pkg.Connections) == 0 {
		//silently exit if the model does not contain any components, relationships, or connections
		return
	}
	ignored := model.ModelDefinitionStatusIgnored
	// 1. Register the model
	model := pkg.Model
	modelstatus := model.Status
	if modelstatus == ignored {
		return
	}
	// Don't register anything else if registrant is not there
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

		// Write SVG for models
		model.Metadata.SvgColor, model.Metadata.SvgWhite, svgCompletePath = WriteAndReplaceSVGWithFileSystemPath(
			model.Metadata.SvgColor,
			model.Metadata.SvgWhite,
			svgComplete,
			rh.svgBaseDir,
			model.Name,
			model.Name,
			true,
		)
		if svgCompletePath != "" {
			model.Metadata.SvgComplete = &svgCompletePath
		}
	}

	model.Registrant.Status = connection.Registered
	_, _, err := rh.regManager.RegisterEntity(model.Registrant, &model)

	// If model cannot be registered, don't register anything else
	if err != nil {
		err = ErrRegisterEntity(err, string(model.Type()), model.DisplayName)
		rh.regErrStore.InsertEntityRegError(model.Registrant.Kind, "", entity.Model, model.Name, err)
		return
	}

	hostname := model.Registrant.Kind

	// Prepare slices to hold successfully registered components, relationships, and connections
	var registeredComponents []component.ComponentDefinition
	var registeredRelationships []relationship.RelationshipDefinition
	var registeredConnections []v1beta1.ConnectionDefinition
	// 2. Register components
	for _, comp := range pkg.Components {
		status := *comp.Status
		if status == component.Ignored {
			continue
		}

		comp.Model = model

		if comp.Styles != nil {
			// Write SVG for components
			comp.Styles.SvgColor, comp.Styles.SvgWhite, comp.Styles.SvgComplete = WriteAndReplaceSVGWithFileSystemPath(
				comp.Styles.SvgColor,
				comp.Styles.SvgWhite,
				comp.Styles.SvgComplete,
				rh.svgBaseDir,
				comp.Model.Name,
				comp.Component.Kind,
				false,
			)
		}

		_, _, err := rh.regManager.RegisterEntity(model.Registrant, &comp)
		if err != nil {
			err = ErrRegisterEntity(err, string(comp.Type()), comp.DisplayName)
			rh.regErrStore.InsertEntityRegError(hostname, model.DisplayName, entity.ComponentDefinition, comp.DisplayName, err)
		} else {
			// Successful registration, add to successfulComponents
			registeredComponents = append(registeredComponents, comp)
		}
	}

	// 3. Register relationships
	for _, rel := range pkg.Relationships {
		rel.Model = model
		_, _, err := rh.regManager.RegisterEntity(model.Registrant, &rel)
		if err != nil {
			err = ErrRegisterEntity(err, string(rel.Type()), string(rel.Kind))
			rh.regErrStore.InsertEntityRegError(hostname, model.DisplayName, entity.RelationshipDefinition, rel.Id.String(), err)
		} else {
			// Successful registration, add to successfulRelationships
			registeredRelationships = append(registeredRelationships, rel)
		}
	}

	// 4. Register connections
	for _, conn := range pkg.Connections {
		conn.Model = model
		_, _, err := rh.regManager.RegisterEntity(model.Registrant, &conn)
		if err != nil {
			err = ErrRegisterEntity(err, string(conn.Type()), conn.Kind)
			rh.regErrStore.InsertEntityRegError(hostname, model.DisplayName, entity.ConnectionDefinition, conn.Kind, err)
		} else {
			// Successful registration, add to successfulConnections
			registeredConnections = append(registeredConnections, conn)
		}
	}

	// Update pkg with only successfully registered components, relationships, and connections
	pkg.Components = registeredComponents
	pkg.Relationships = registeredRelationships
	pkg.Connections = registeredConnections
	pkg.Model = model
	// Store the successfully registered PackagingUnit
	rh.PkgUnits = append(rh.PkgUnits, pkg)
}
