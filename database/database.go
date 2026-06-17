//go:build !js

package database

import (
	"fmt"
	"sync"

	"gorm.io/driver/postgres"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func (h *Handler) DBClose() error {
	db, err := h.DB.DB()
	if err != nil {
		return err
	}
	err = db.Close() //It ensures that all writes have completed and the database is not corrupted.
	if err != nil {
		return err
	}
	return nil
}

func New(opts Options) (Handler, error) {
	switch opts.Engine {
	case POSTGRES:
		dsn := fmt.Sprintf("host=%s user=%s password=%s port=%s", opts.Host, opts.Username, opts.Password, opts.Port)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return Handler{}, ErrDatabaseOpen(err)
		}
		return Handler{
			db,
			&sync.Mutex{},
		}, nil
	case SQLITE:
		config := &gorm.Config{}
		if opts.Logger != nil {
			config.Logger = opts.Logger.DatabaseLogger()
		}

		db, err := gorm.Open(sqlite.Open(opts.Filename), config)
		if err != nil {
			return Handler{}, ErrDatabaseOpen(err)
		}

		return Handler{
			db,
			&sync.Mutex{},
		}, nil
	}

	return Handler{}, ErrNoneDatabase
}
