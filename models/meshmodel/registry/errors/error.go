package errors

import (
	meshkiterrors "github.com/meshery/meshkit/errors"
)

const (
	ErrGetByIdCode = "meshkit-11262"
)

func ErrGetById(err error, id string) error {
	return meshkiterrors.New(
		ErrGetByIdCode,
		meshkiterrors.Alert,
		[]string{"Failed to get the entity with the given ID: " + id},
		[]string{err.Error()},
		[]string{"Entity with the given ID may not be present in the registry", "Registry might be inaccessible at the moment"},
		[]string{"Check if your ID is correct", "If the registry is inaccesible, please try again after some time"},
	)
}
