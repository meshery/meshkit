package database

import (
	"fmt"
	"sync"

	"github.com/meshery/meshkit/logger"
	"gorm.io/driver/postgres"
	sqlite "gorm.io/driver/sqlite"
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
	Database string `json:"database,omitempty"`
	SslMode  string `json:"sslmode,omitempty"`
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

type Handler struct {
	*gorm.DB
	*sync.Mutex
	// Implement methods if necessary
}

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

func buildPostgresDSN(opts Options) string {
	dsn := fmt.Sprintf("host=%s user=%s password=%s port=%s dbname=%s", opts.Host, opts.Username, opts.Password, opts.Port, opts.Database)
	if opts.SslMode != "" {
		dsn = fmt.Sprintf("%s sslmode=%s", dsn, opts.SslMode)
	}
	return dsn
}

func New(opts Options) (Handler, error) {
	switch opts.Engine {
	case POSTGRES:
		if opts.Database == "" {
			return Handler{}, ErrNoneDatabase
		}
		dsn := buildPostgresDSN(opts)
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
