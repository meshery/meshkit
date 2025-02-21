package entity

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrUpdateEntityStatusCode = "meshkit-11243"
)

func ErrUpdateEntityStatus(err error, entity string, status EntityStatus) error {
	return errors.New(ErrUpdateEntityStatusCode, errors.Alert, 
		[]string{fmt.Sprintf("Unable to update %s to %s.", entity, status)}, 
		[]string{err.Error()}, 
		[]string{"Entity status update failed due to internal error."}, 
		[]string{"Please try again. If the issue persists, check the entity and status values."})
}
