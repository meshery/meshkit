package models

import "github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"

// anything that can be validated is a Validator
type Validator interface {
	Validate([]byte) error
}

// Package related interfaces

type Package interface {
	GenerateComponents() ([]v1alpha1.Component, error)
}

type PackageManager interface {
	GetPackage() (Package, error)
}
