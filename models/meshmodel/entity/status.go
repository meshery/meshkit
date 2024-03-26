package entity

import "github.com/layer5io/meshkit/database"

type EntityStatus string

const (
	Ignored   EntityStatus = "ignored"
	Enabled   EntityStatus = "enabled"
	Duplicate EntityStatus = "duplicate"
)

type Status interface {
	UpdateStatus(db *database.Handler, status EntityStatus) error
}
