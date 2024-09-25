package models

import "github.com/meshery/schemas/models/v1beta1/component"

// anything that can be validated is a Validator
type Validator interface {
	Validate([]byte) error
}

// An entity that is used to expose a particular
// system's capabilities in Meshery
// A Package should have all the information that we need to generate the components
type Package interface {
	GenerateComponents() ([]component.ComponentDefinition, error)
	GetVersion() string
	GetSourceURL() string
	GetName() string
}

// Supports pulling packages from Artifact Hub and other sources like Docker Hub.
// Should envelope Meshery Application importer - to be implemented
type PackageManager interface {
	GetPackage() (Package, error)
}
