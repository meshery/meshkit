package v1beta1

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/schemas/models/v1beta1/connection"
	v1beta1 "github.com/meshery/schemas/models/v1beta1/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConnectionDefinition represents a connection definition entity
type ConnectionDefinition struct {
	ID         uuid.UUID               `json:"-" gorm:"primarykey"`
	Kind       string                  `json:"kind,omitempty" yaml:"kind"`
	Version    string                  `json:"version,omitempty" yaml:"version"`
	ModelID    uuid.UUID               `json:"-" gorm:"column:modelID"`
	Model      v1beta1.ModelDefinition `json:"model"`
	SubType    string                  `json:"subType" yaml:"subType"`
	Connection connection.Connection   `json:"connection" yaml:"connection"`
	CreatedAt  time.Time               `json:"-"`
	UpdatedAt  time.Time               `json:"-"`
}

func (c ConnectionDefinition) GetID() uuid.UUID {
	return c.ID
}

func (c *ConnectionDefinition) GenerateID() (uuid.UUID, error) {
	return uuid.NewV4()
}

func (c ConnectionDefinition) Type() entity.EntityType {
	return entity.ConnectionDefinition
}

func (c *ConnectionDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, definition version: %s, name: %s, model: %s, version: %s", c.Type(), c.Version, c.Kind, c.Model.Name, c.Model.Version)
}

func (c *ConnectionDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	id, idErr := c.GenerateID()
	if idErr != nil {
		return uuid.UUID{}, idErr
	}
	c.ID = id

	// Wrap both database operations in a transaction for atomicity
	err := db.Transaction(func(tx *gorm.DB) error {
		// Create a new handler for the transaction scope
		txHandler := &database.Handler{DB: tx, Mutex: db.Mutex}

		// Create the Model first
		mid, txErr := c.Model.Create(txHandler, hostID)
		if txErr != nil {
			return txErr
		}
		c.ModelID = mid

		// Create the ConnectionDefinition
		if txErr := tx.Omit(clause.Associations).Create(&c).Error; txErr != nil {
			return txErr
		}
		return nil
	})

	if err != nil {
		return uuid.UUID{}, err
	}
	return c.ID, nil
}

func (c *ConnectionDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	// TODO: Implement status update logic for connection definitions
	return nil
}
