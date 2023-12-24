package utils

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

type IPackageManager interface {
	UpdatePackageData() error
	GenerateComponents() ([]v1alpha1.ComponentDefinition, error)
}
