package controllers

import (
	"github.com/meshery/meshkit/errors"
)

var (
	ErrGetControllerStatusCode          = "meshkit-11141"
	ErrDeployControllerCode             = "meshkit-11142"
	ErrGetControllerPublicEndpointCode  = "meshkit-11143"
	ErrGetControllerEndpointForPortCode = "meshkit-11319"
)

func ErrGetControllerStatus(err error) error {
	return errors.New(ErrGetControllerStatusCode, errors.Alert, []string{"Error getting the status of the meshery controller"}, []string{err.Error()}, []string{"Controller may not be healthy or not deployed"}, []string{"Make sure the controller is deployed and healthy"})
}

func ErrDeployController(err error) error {
	return errors.New(ErrDeployControllerCode, errors.Alert, []string{"Error deploying Meshery Operator"}, []string{err.Error()}, []string{"Meshery Server could not connect to the Kubernetes cluster. Meshery Operator  was not deployed", "Insufficient file permission to read kubeconfig"}, []string{"Verify that the available kubeconfig is accessible by Meshery Server - verify sufficient file permissions (only needs read permission)"})
}

func ErrGetControllerPublicEndpoint(err error) error {
	return errors.New(ErrGetControllerPublicEndpointCode, errors.Alert, []string{"Could not get the public endpoint of the controller"}, []string{err.Error()}, []string{"Client configuration may not be valid"}, []string{"Make sure the client configuration is valid"})
}

func ErrGetControllerEndpointForPort(err error) error {
	return errors.New(ErrGetControllerEndpointForPortCode, errors.Alert, []string{"Could not get endpoint for port of the controller"}, []string{err.Error()}, []string{"Client configuration may not be valid"}, []string{"Make sure the client configuration is valid"})
}
