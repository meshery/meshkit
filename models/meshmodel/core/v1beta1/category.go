package v1beta1

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"gorm.io/gorm"
)

var categoryCreationLock sync.Mutex //Each model will perform a check and if the category already doesn't exist, it will create a category. This lock will make sure that there are no race conditions.

// swagger:response Category
type Category struct {
	ID       uuid.UUID              `json:"-"`
	Name     string                 `json:"name" gorm:"name"`
	Metadata map[string]interface{} `json:"metadata"  yaml:"metadata" gorm:"type:bytes;serializer:json"`
}

func (c Category) TableName() string {
	return "category_dbs"
}

// "Uncategorized" is assigned when Category is empty in the component definitions.
const DefaultCategory = "Uncategorized"

func (cat Category) Type() entity.EntityType {
	return entity.Category
}

func (cat *Category) GenerateID() (uuid.UUID, error) {
	byt, err := json.Marshal(cat)
	if err != nil {
		return uuid.UUID{}, err
	}
	catID := uuid.NewSHA1(uuid.UUID{}, byt)
	return catID, nil
}

func (cat Category) GetID() uuid.UUID {
	return cat.ID
}

func (cat *Category) GetEntityDetail() string {
	return fmt.Sprintf("name: %s", cat.Name)
}

func (cat *Category) Create(db *database.Handler, _ uuid.UUID) (uuid.UUID, error) {
	if cat.Name == "" {
		cat.Name = DefaultCategory
	}

	catID, err := cat.GenerateID()
	if err != nil {
		return catID, err
	}
	var category Category
	categoryCreationLock.Lock()
	defer categoryCreationLock.Unlock()
	err = db.First(&category, "id = ?", catID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if err == gorm.ErrRecordNotFound { //The category is already not present and needs to be inserted
		cat.ID = catID
		err = db.Create(&cat).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		return cat.ID, nil
	}
	return category.ID, nil
}

func (m *Category) UpdateStatus(db database.Handler, status entity.EntityStatus) error {
	return nil
}
