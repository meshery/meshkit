package database

import (
	"sync"

	"github.com/meshery/meshkit/logger"
	"gorm.io/gorm"
)

const (
	POSTGRES = "postgres"
	SQLITE   = "sqlite"
)

type Options struct {
	Username string `json:"username,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	Filename string `json:"filename,omitempty"`
	Engine   string `json:"engine,omitempty"`
	Logger   logger.Handler
}

type Model struct {
	ID        string `json:"id,omitempty" gorm:"primarykey"`
	CreatedAt string `json:"createdAt,omitempty" gorm:"index"`
	UpdatedAt string `json:"updatedAt,omitempty" gorm:"index"`
	DeletedAt string `json:"deletedAt,omitempty" gorm:"index"`
}

// TODO: Relocate Handler to a !js-gated file once schemas/*_helper.go stop
// referencing it, to keep gorm out of wasm builds.
type Handler struct {
	*gorm.DB
	*sync.Mutex
	// Implement methods if necessary
}
