package v1beta1

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/models/meshmodel/entity"
	"github.com/meshery/meshkit/utils"
	schemav1beta1 "github.com/meshery/schemas/models/v1beta1"
	"github.com/meshery/schemas/models/v1beta1/connection"
	v1beta1 "github.com/meshery/schemas/models/v1beta1/model"
	"gorm.io/gorm/clause"
)

// ConnectionSchemaVersion is imported from the schemas repo
// This ensures consistency with the schema definition
const ConnectionSchemaVersion = schemav1beta1.ConnectionSchemaVersion

// ConnectionDefinition wraps the Connection from schemas to implement the Entity interface
// This allows us to use Connection entities directly in PackagingUnit as requested by maintainer
type ConnectionDefinition struct {
	connection.Connection
	ModelID   uuid.UUID               `json:"-" gorm:"column:modelID"`
	Model     v1beta1.ModelDefinition `json:"model"`
	CreatedAt time.Time               `json:"-"`
	UpdatedAt time.Time               `json:"-"`
}

func (c ConnectionDefinition) GetID() uuid.UUID {
	return c.ID
}

func (c *ConnectionDefinition) GenerateID() (uuid.UUID, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func (c ConnectionDefinition) EntityType() entity.EntityType {
	return entity.ConnectionDefinition
}

func (c *ConnectionDefinition) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, name: %s, kind: %s, model: %s, version: %s", c.EntityType(), c.Name, c.Kind, c.Model.Name, c.Model.Version)
}

func (c *ConnectionDefinition) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	id, err := c.GenerateID()
	if err != nil {
		return uuid.UUID{}, err
	}
	c.ID = id

	mid, err := c.Model.Create(db, hostID)
	if err != nil {
		return uuid.UUID{}, err
	}
	c.ModelID = mid
	err = db.Omit(clause.Associations).Create(&c).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return c.ID, nil
}

func (c *ConnectionDefinition) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	return nil
}

func (c ConnectionDefinition) WriteConnectionDefinition(connectionDirPath string) error {
	connectionPath := filepath.Join(connectionDirPath, c.Kind+".json")
	err := utils.WriteJSONToFile[ConnectionDefinition](connectionPath, c)
	return err
}
