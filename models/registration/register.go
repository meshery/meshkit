package registration

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha2"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	meshmodel "github.com/layer5io/meshkit/models/meshmodel/registry"
)

// packaingUnit is the representation of the atomic unit that can be registered into the capabilities registry
type packagingUnit struct {
	model         v1beta1.Model
	components    []v1beta1.ComponentDefinition
	relationships []v1alpha2.RelationshipDefinition
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
func (rh *RegistrationHelper) Register(entity RegisterableEntity) error {
	// get the packaging units
	pu, err := entity.PkgUnit(rh.regErrStore)
	if err != nil {
		// given input is not a valid model, or could not walk the directory
		return err
	}
	// fmt.Printf("Packaging Unit: Model name: %s, comps: %d, rels: %d\n", pu.model.Name, len(pu.components), len(pu.relationships))
	return rh.register(pu)
}

/*
register will return an error if it is not able to register the `model`.
If there are errors when registering other entities, they are handled properly but does not stop the registration process.
*/
func (rh *RegistrationHelper) register(pkg packagingUnit) error {
	// 1. Register the model
	model := pkg.model

	// Dont register anything else if registrant is not there
	if model.Registrant.Hostname == "" {
		err := ErrMissingRegistrant(model.Name)
		rh.regErrStore.InsertEntityRegError(model.Registrant.Hostname, "", entity.Model, model.Name, err)
		return err
	}
	writeAndReplaceSVGWithFileSystemPath(model.Metadata, rh.svgBaseDir, model.Name, model.Name) //Write SVG for models
	_, _, err := rh.regManager.RegisterEntity(
		v1beta1.Host{Hostname: model.Registrant.Hostname},
		&model,
	)

	// If model cannot be registered, don't register anything else
	if err != nil {
		err = ErrRegisterEntity(err, string(model.Type()), model.DisplayName)
		rh.regErrStore.InsertEntityRegError(model.Registrant.Hostname, "", entity.Model, model.Name, err)
		return err
	}

	hostname := model.Registrant.Hostname
	modelName := model.Name
	// 2. Register components
	for _, comp := range pkg.components {
		comp.Model = model
		writeAndReplaceSVGWithFileSystemPath(comp.Metadata, rh.svgBaseDir, comp.Model.Name, comp.Component.Kind) //Write SVG on components
		_, _, err := rh.regManager.RegisterEntity(
			v1beta1.Host{Hostname: hostname},
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
		_, _, err := rh.regManager.RegisterEntity(v1beta1.Host{
			Hostname: hostname,
		}, &rel)
		if err != nil {
			err = ErrRegisterEntity(err, string(rel.Type()), rel.Kind)
			rh.regErrStore.InsertEntityRegError(hostname, modelName, entity.RelationshipDefinition, rel.ID.String(), err)
		}
	}
	return nil
}
