package entity

import registryentity "github.com/meshery/meshkit/models/registry/entity"

const ErrUpdateEntityStatusCode = registryentity.ErrUpdateEntityStatusCode

// ErrUpdateEntityStatus is a deprecated wrapper for registryentity.ErrUpdateEntityStatus.
// Deprecated: use registryentity.ErrUpdateEntityStatus instead.
func ErrUpdateEntityStatus(err error, entity string, status EntityStatus) error {
	return registryentity.ErrUpdateEntityStatus(err, entity, status)
}
