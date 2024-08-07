package registration

import (
	"github.com/layer5io/meshkit/models/meshmodel/entity"
)

/* 
    RegistrationErrorStore stores all the errors that does not break the registration process, but have to be reported nevertheless.
*/
type RegistrationErrorStore interface {
    AddInvalidDefinition(string,error)
    InsertEntityRegError(hostname string, modelName string, entityType entity.EntityType, entityName string, err error)
}

// Anything that can be parsed into a packagingUnit is a RegisterableEntity in Meshery server
type RegisterableEntity interface {
   /*
		1. `err` - this is a breaking error, which signifies that the given entity is invalid and cannot be registered 
		2. Errors encountered while parsing items into meshmodel entites are stored in the RegistrationErrorStore
	*/
	PkgUnit(RegistrationErrorStore) (packagingUnit, error)
}


