package v1beta1

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/layer5io/meshkit/utils"
	"gorm.io/gorm"
)

var modelCreationLock sync.Mutex //Each component/relationship will perform a check and if the model already doesn't exist, it will create a model. This lock will make sure that there are no race conditions.

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

type model struct {
	Version string `json:"version,omitempty" yaml:"version"`
}

// swagger:response Model
type Model struct {
	ID uuid.UUID `json:"id,omitempty" yaml:"-"`
	VersionMeta
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName" gorm:"modelDisplayName"`
	Description string                 `json:"description" gorm:"description"`
	Status      entity.EntityStatus            `json:"status" gorm:"status"`
	Registrant  registry.Hostv1beta1   `json:"registrant" gorm:"registrant"` // to be Connection
	Category    Category               `json:"category"`
	SubCategory string                 `json:"subCategory" gorm:"subCategory"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"modelMetadata"`
	Model       model                  `json:"model,omitempty" gorm:"model"`
	// Components      []ComponentDefinitionDB    `json:"components"`
	// Relationships   []RelationshipDefinitionDB `json:"relationships"`
}

type ModelDB struct {
	ID uuid.UUID `json:"id"`
	VersionMeta
	Name         string      `json:"modelName" gorm:"modelName"`
	DisplayName  string      `json:"modelDisplayName" gorm:"modelDisplayName"`
	Description  string      `json:"description" gorm:"description"`
	Status       entity.EntityStatus `json:"status" gorm:"status"`
	RegistrantID uuid.UUID   `json:"hostID" gorm:"hostID"`
	CategoryID   uuid.UUID   `json:"-" gorm:"categoryID"`
	SubCategory  string      `json:"subCategory" gorm:"subCategory"`
	Metadata     []byte      `json:"modelMetadata" gorm:"modelMetadata"`
	Model        model       `json:"model,omitempty" gorm:"model"`
}

func (m Model) Type() entity.EntityType {
	return entity.Model
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
		mdb.Status = entity.Enabled
		err = db.Create(&mdb).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		return mdb.ID, nil
	}
	return model.ID, nil
}

func (m *Model) UpdateStatus(db database.Handler, status entity.EntityStatus) error {
	err := db.Model(&ModelDB{}).Where("id = ?", m.ID).Update("status", status).Error
	if err != nil {
		return entity.ErrUpdateEntityStatus(err, string(m.Type()), status.String())
	}
	return nil
}

func (c *Model) GetModelDB() (cmd ModelDB) {
	// cmd.ID = c.ID id will be assigned by the database itself don't use this, as it will be always uuid.nil, because id is not known when comp gets generated.
	// While database creates an entry with valid primary key but to avoid confusion, it is disabled and accidental assignment of custom id.
	cmd.VersionMeta = c.VersionMeta
	cmd.Name = c.Name
	cmd.DisplayName = c.DisplayName
	cmd.Description = c.Description
	cmd.Status = c.Status
	cmd.RegistrantID = c.Registrant.ID
	cmd.CategoryID = c.Category.ID
	cmd.SubCategory = c.SubCategory
	cmd.Metadata, _ = json.Marshal(c.Metadata)
	cmd.Model = c.Model
	return
}

// is reg should be passed as param?
func (cmd *ModelDB) GetModel(cat Category, reg registry.Hostv1beta1) (c Model) {
	c.ID = cmd.ID
	c.VersionMeta = cmd.VersionMeta
	c.Name = cmd.Name
	c.DisplayName = cmd.DisplayName
	c.Description = cmd.Description
	c.Status = cmd.Status
	c.Registrant = reg
	c.Category = cat
	c.SubCategory = cmd.SubCategory
	_ = json.Unmarshal(cmd.Metadata, &c.Metadata)
	c.Model = cmd.Model
	// c.Components = make([]ComponentDefinitionDB, 0)
	// c.Relationships = make([]RelationshipDefinitionDB, 0)
	return
}

func (c Model) WriteModelDefinition(modelDefPath string) error {
	err := utils.CreateDirectory(modelDefPath)
	if err != nil {
		return err
	}

	modelFilePath := filepath.Join(modelDefPath, "model.json")
	err = utils.WriteJSONToFile[Model](modelFilePath, c)
	if err != nil {
		return err
	}
	return nil
}
