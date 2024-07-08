package v1beta1

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ModelSchemaVersion = "models.meshery.io/v1beta1"

var modelCreationLock sync.Mutex //Each component/relationship will perform a check and if the model already doesn't exist, it will create a model. This lock will make sure that there are no race conditions.

type ModelEntity struct {
	Version string `json:"version,omitempty" yaml:"version"`
}

// swagger:response Model
type Model struct {
	ID uuid.UUID `json:"id"`
	VersionMeta
	Name          string                 `json:"name" gorm:"modelName"`
	DisplayName   string                 `json:"displayName"`
	Description   string                 `json:"description" gorm:"description"`
	Status        entity.EntityStatus    `json:"status" gorm:"status"`
	RegistrantID  uuid.UUID              `json:"hostID" gorm:"column:host_id"` // make as a foreign refer to host's table
	Registrant    Host                   `json:"registrant" gorm:"foreignKey:RegistrantID;references:ID"`
	CategoryID    uuid.UUID              `json:"-" gorm:"categoryID"`
	Category      Category               `json:"category" gorm:"foreignKey:CategoryID;references:ID"`
	SubCategory   string                 `json:"subCategory" gorm:"subCategory"`
	Metadata      map[string]interface{} `json:"metadata" gorm:"type:bytes;serializer:json"`
	Model         ModelEntity            `json:"model,omitempty" gorm:"model;type:bytes;serializer:json"`
	Components    []ComponentDefinition  `json:"components" gorm:"-"`
	Relationships interface{}            `json:"relationships" gorm:"-"`
}

func (m Model) TableName() string {
	return "model_dbs"
}

func (m Model) Type() entity.EntityType {
	return entity.Model
}

func (m *Model) GenerateID() (uuid.UUID, error) {
	modelIdentifier := Model{
		Registrant:  m.Registrant,
		VersionMeta: m.VersionMeta,
		Name:        m.Name,
		Model: ModelEntity{
			Version: m.Model.Version,
		},
	}
	byt, err := json.Marshal(modelIdentifier)
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.NewSHA1(uuid.UUID{}, byt), nil
}

func (m Model) GetID() uuid.UUID {
	return m.ID
}

func (m *Model) GetEntityDetail() string {
	return fmt.Sprintf("type: %s, model: %s, definition version: %s, version: %s", m.Type(), m.Name, m.Version, m.Model.Version)
}

func (m *Model) Create(db *database.Handler, hostID uuid.UUID) (uuid.UUID, error) {
	modelID, err := m.GenerateID()
	if err != nil {
		return modelID, err
	}

	var model Model
	if m.Name == "" {
		return uuid.UUID{}, fmt.Errorf("empty or invalid model name passed")
	}
	modelCreationLock.Lock()
	defer modelCreationLock.Unlock()
	err = db.First(&model, "id = ? and host_id = ?", modelID, hostID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if err == gorm.ErrRecordNotFound { //The model is already not present and needs to be inserted
		id, err := m.Category.Create(db, hostID)
		if err != nil {
			return uuid.UUID{}, err
		}
		m.ID = modelID
		m.CategoryID = id
		m.RegistrantID = hostID
		m.Status = entity.Enabled
		err = db.Omit(clause.Associations).Create(&m).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		// register model inside registries table
		err = registerModel(db, hostID, modelID)
		if err != nil {
			return uuid.UUID{}, err
		}
		return m.ID, nil
	}
	return model.ID, nil
}

func (m *Model) UpdateStatus(db *database.Handler, status entity.EntityStatus) error {
	err := db.Model(&Model{}).Where("id = ?", m.ID).Update("status", status).Error
	if err != nil {
		return entity.ErrUpdateEntityStatus(err, string(m.Type()), status)
	}
	return nil
}

// WriteModelDefinition writes out the model to the given `modelDefPath` in the `outputType` format.
// `outputType` can be `yaml` or `json`.
func (c Model) WriteModelDefinition(modelDefPath string, outputType string) error {
	err := utils.CreateDirectory(modelDefPath)
	if err != nil {
		return err
	}
	var modelFilePath string
	if outputType == "json" {
		modelFilePath = filepath.Join(modelDefPath, "model.json")
		err = utils.WriteJSONToFile[Model](modelFilePath, c)
	}
	if outputType == "yaml" {
		modelFilePath = filepath.Join(modelDefPath, "model.yaml")
		err = utils.WriteYamlToFile[Model](modelFilePath, c)
	}
	if err != nil {
		return err
	}
	return nil
}

// Registers models into registries table.
func registerModel(db *database.Handler, regID, modelID uuid.UUID) error {
	var count int64
	err := db.Table("registries").Where("registrant_id = ?", regID).Where("type = ?", "model").Where("entity = ?", modelID).Count(&count).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if count == 0 {
		err = db.Exec("INSERT INTO registries (registrant_id, entity, type) VALUES (?,?,?)", regID, modelID, "model").Error
		if err != nil {
			return err
		}
	}
	return nil
}
