package compgen

import (
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

// responsible for generating components using different sorts of manifests
type ComponentsGenerator interface {
	Generate() ([]v1alpha1.Component, error)
}
