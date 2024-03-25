package entity

import "github.com/layer5io/meshkit/database"

type EntityStatus int 

const (
	Ignored EntityStatus = iota
	Enabled
	Duplicate
)

func(e EntityStatus) String() string {
	switch e {
	case Ignored:
		return "ignored"
	case Duplicate:
		return "duplicate"
	case Enabled:
		fallthrough		
	default:
		return "enabled"
	}
	
}

type Status interface {
	UpdateStatus(db *database.Handler, status EntityStatus) error 
}