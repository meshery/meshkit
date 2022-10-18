package github

import "github.com/layer5io/meshkit/errors"

const ErrComponentGenerateCode = "11095"
const ErrGetGHPackageCode = "11096"

func ErrComponentGenerate(err error) error {
	return errors.New(ErrComponentGenerateCode, errors.Alert, []string{"failed to generate components for the package"}, []string{err.Error()}, []string{}, []string{"Make sure that the package is compatible"})
}

func ErrGetGHPackage(err error) error {
	return errors.New(ErrGetGHPackageCode, errors.Alert, []string{"Could not get the Github package with the given name"}, []string{err.Error()}, []string{""}, []string{"make sure that the package exists"})
}
