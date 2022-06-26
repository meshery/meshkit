package controllers

import (
	"github.com/layer5io/meshkit/errors"
)

var (
	ErrGetControllerStatusCode         = "11080"
	ErrDeployControllerCode            = "11081"
	ErrGetControllerPublicEndpointCode = "11082"
)

func ErrGetControllerStatus(err error) error {
	return errors.New(ErrGetControllerStatusCode, errors.Alert, []string{"Error getting the status of the meshery controller"}, []string{err.Error()}, []string{"Controller may not be healthy or not deployed"}, []string{"Make sure the controller is deployed and healthy"})
}

func ErrDeployController(err error) error {
	return errors.New(ErrDeployControllerCode, errors.Alert, []string{"Error deploying meshery controller"}, []string{err.Error()}, []string{"Controller configuration may not be valid"}, []string{"Make sure the controller configuration is valid"})
}

func ErrGetControllerPublicEndpoint(err error) error {
	return errors.New(ErrGetControllerPublicEndpointCode, errors.Alert, []string{"Could not get the public endpoint of the controller"}, []string{err.Error()}, []string{"Client configuration may not be valid"}, []string{"Make sure the client configuration is valid"})
}
