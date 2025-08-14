package v1beta1

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofrs/uuid"
	"github.com/meshery/meshkit/database"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas/models/v1beta1/connection"
	v1beta1 "github.com/meshery/schemas/models/v1beta1/model"
	"gorm.io/gorm/clause"

	"github.com/meshery/meshkit/models/meshmodel/entity"
)

// swagger:response ConnectionDefinition
type ConnectionDefinition struct {
	ID         uuid.UUID               `json:"-" gorm:"primaryKey"`
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
	c.ID, _ = c.GenerateID()

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
