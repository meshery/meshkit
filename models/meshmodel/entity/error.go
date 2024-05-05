package entity

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrUpdateEntityStatusCode = ""
)

func ErrUpdateEntityStatus(err error, entity string, status EntityStatus) error {
	return errors.New(ErrUpdateEntityStatusCode, errors.Alert, []string{fmt.Sprintf("unable to update %s to %s", entity, status)}, []string{err.Error()}, []string{}, []string{})
}
