package kubernetes

import "github.com/layer5io/meshkit/errors"

func ErrApplyManifest(err error) error {
	return errors.NewDefault(errors.ErrApplyManifest, "Error Applying manifest: "+err.Error())
}
