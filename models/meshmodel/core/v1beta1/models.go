package v1beta1

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	types "github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"gorm.io/gorm"
)

var modelCreationLock sync.Mutex //Each component/relationship will perform a check and if the model already doesn't exist, it will create a model. This lock will make sure that there are no race conditions.

type ModelStatus string

type ModelFilter struct {
	Name        string
	Registrant  string //name of the registrant for a given model
	DisplayName string //If Name is already passed, avoid passing Display name unless greedy=true, else the filter will translate to an AND returning only the models where name and display name match exactly. Ignore, if this behavior is expected.
	Greedy      bool   //when set to true - instead of an exact match, name will be prefix matched. Also an OR will be performed of name and display_name
	Version     string
	Category    string
	OrderOn     string
	Sort        string //asc or desc. Default behavior is asc
	Limit       int    //If 0 or unspecified then all records are returned and limit is not used
	Offset      int
	Annotations string //When this query parameter is "true", only models with the "isAnnotation" property set to true are returned. When  this query parameter is "false", all models except those considered to be annotation models are returned. Any other value of the query parameter results in both annoations as well as non-annotation models being returned.

	// When these are set to true, we also retrieve components/relationships associated with the model.
	Components    bool
	Relationships bool
	Status        string
}

// swagger:response Model
type Model struct {
	ID              uuid.UUID              `json:"id,omitempty" yaml:"-"`
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"displayName" gorm:"modelDisplayName"`
	Version         string                 `json:"version"`
	Status          ModelStatus            `json:"status" gorm:"status"`
	Registrant      registry.Hostv1beta1   `json:"registrant" gorm:"registrant"` // to be Connection
	Category        Category               `json:"category"`
	Metadata        map[string]interface{} `json:"metadata" yaml:"modelMetadata"`
	// Components      []ComponentDefinitionDB    `json:"components"`
	// Relationships   []RelationshipDefinitionDB `json:"relationships"`
}

type ModelDB struct {
	ID           uuid.UUID   `json:"id"`
	CategoryID   uuid.UUID   `json:"-" gorm:"categoryID"`
	RegistrantID uuid.UUID   `json:"hostID" gorm:"hostID"`
	Name         string      `json:"modelName" gorm:"modelName"`
	Version      string      `json:"version"`
	DisplayName  string      `json:"modelDisplayName" gorm:"modelDisplayName"`
	SubCategory  string      `json:"subCategory" gorm:"subCategory"`
	Metadata     []byte      `json:"modelMetadata" gorm:"modelMetadata"`
	Status       ModelStatus `json:"status" gorm:"status"`
}

func (m Model) Type() types.EntityType {
	return types.Model
}
func (m Model) GetID() uuid.UUID {
	return m.ID
}

func (m *Model) Create(db *database.Handler) (uuid.UUID, error) {

	hostID, err := m.Registrant.Create(db)
	if err != nil {
		return uuid.UUID{}, err
	}

	byt, err := json.Marshal(m)
	if err != nil {
		return uuid.UUID{}, err
	}
	modelID := uuid.NewSHA1(uuid.UUID{}, byt)
	var model ModelDB
	if m.Name == "" {
		return uuid.UUID{}, fmt.Errorf("empty or invalid model name passed")
	}
	modelCreationLock.Lock()
	defer modelCreationLock.Unlock()
	err = db.First(&model, "id = ? and hostID = ?", modelID, hostID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if err == gorm.ErrRecordNotFound { //The model is already not present and needs to be inserted
		id, err := m.Category.Create(db)
		if err != nil {
			return uuid.UUID{}, err
		}
		m.ID = modelID
		mdb := m.GetModelDB()
		mdb.CategoryID = id
		mdb.RegistrantID = hostID
		mdb.Status = "registered"
		err = db.Create(&mdb).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		return mdb.ID, nil
	}
	return model.ID, nil
}

func (c *Model) GetModelDB() (cmd ModelDB) {
	cmd.ID = c.ID
	cmd.DisplayName = c.DisplayName
	cmd.Status = c.Status
	cmd.Name = c.Name
	cmd.Version = c.Version
	cmd.Metadata, _ = json.Marshal(c.Metadata)
	return
}

func (cmd *ModelDB) GetModel(cat Category) (c Model) {
	c.ID = cmd.ID
	c.Category = cat
	c.DisplayName = cmd.DisplayName
	c.Status = cmd.Status
	c.Name = cmd.Name
	c.Version = cmd.Version
	// c.Components = make([]ComponentDefinitionDB, 0)
	// c.Relationships = make([]RelationshipDefinitionDB, 0)
	_ = json.Unmarshal(cmd.Metadata, &c.Metadata)
	return
}
