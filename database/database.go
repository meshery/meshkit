package database

import (
	"github.com/layer5io/meshkit/logger"
	sqlite "gorm.io/driver/sqlite"
	gormpkg "gorm.io/gorm"
)

const (
	POSTGRES = "postgres"
	SQLITE   = "sqlite"
)

type Options struct {
	Filename string `json:"filename,omitempty"`
	Engine   string `json:"engine,omitempty"`
	Logger   logger.Handler
}

type Model struct {
	ID        string `json:"id,omitempty" gorm:"primarykey"`
	CreatedAt string `json:"created_at,omitempty" gorm:"index"`
	UpdatedAt string `json:"updated_at,omitempty" gorm:"index"`
	DeletedAt string `json:"deleted_at,omitempty" gorm:"index"`
}

type Handler struct {
	*gormpkg.DB
	// Implement methods if necessary
}

func New(opts Options) (Handler, error) {
	switch opts.Engine {
	case POSTGRES:
		return Handler{}, ErrNoneDatabase
	case SQLITE:
		config := &gormpkg.Config{}
		if opts.Logger != nil {
			config.Logger = opts.Logger.DatabaseLogger()
		}

		db, err := gormpkg.Open(sqlite.Open(opts.Filename), config)
		if err != nil {
			return Handler{}, ErrDatabaseOpen(err)
		}
		return Handler{
			db,
		}, nil
	}

	return Handler{}, ErrNoneDatabase
}
