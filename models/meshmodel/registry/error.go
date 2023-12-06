package registry

import (
	"github.com/google/uuid"
	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

var NonImportModel v1alpha1.EntitySummary
var ModelCount map[uuid.UUID]int
var RegistryCount map[string]int

func init() {
	ModelCount = make(map[uuid.UUID]int)
	RegistryCount = make(map[string]int)
}

var (
	ErrUnknownHostCode = "11097"
)

func ErrUnknownHost(err error) error {
	return errors.New(ErrUnknownHostCode, errors.Alert, []string{"host is not supported"}, []string{err.Error()}, []string{"The component's host is not supported by the version of server you are running"}, []string{"Try upgrading to latest available version"})
}
func onModelError(reg Registry) {
	ModelCount[reg.Entity]++
}
func onRegistrantError(h Host) {
	RegistryCount[h.Hostname]++
}
func onEntityError(en Entity) {
	switch en.(type) {
	case v1alpha1.ComponentDefinition:
		NonImportModel.Components++
	case v1alpha1.RelationshipDefinition:
		NonImportModel.Relationships++
	case v1alpha1.PolicyDefinition:
		NonImportModel.Policies++
	}

}
