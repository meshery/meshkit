package v1alpha1

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"gorm.io/gorm"
)

var categoryCreationLock sync.Mutex //Each model will perform a check and if the category already doesn't exist, it will create a category. This lock will make sure that there are no race conditions.

// swagger:response Category
type Category struct {
	ID       uuid.UUID              `json:"-"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata" yaml:"metadata"`
}
type CategoryDB struct {
	ID       uuid.UUID `json:"-"`
	Name     string    `json:"categoryName" gorm:"categoryName"`
	Metadata []byte    `json:"categoryMetadata" gorm:"categoryMetadata"`
}
type CategoryFilter struct {
	Name    string
	OrderOn string
	Sort    string //asc or desc. Default behavior is asc
	Limit   int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset  int
}

const DefaultCategory = "Miscellaneous"

// Create the filter from map[string]interface{}
func (cf *CategoryFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}
func CreateCategory(db *database.Handler, cat Category) (uuid.UUID, error) {
	if cat.Name == "" {
		cat.Name = DefaultCategory
	}
	byt, err := json.Marshal(cat)
	if err != nil {
		return uuid.UUID{}, err
	}
	catID := uuid.NewSHA1(uuid.UUID{}, byt)
	var category CategoryDB
	categoryCreationLock.Lock()
	defer categoryCreationLock.Unlock()
	err = db.First(&category, "id = ?", catID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return uuid.UUID{}, err
	}
	if err == gorm.ErrRecordNotFound { //The category is already not present and needs to be inserted
		cat.ID = catID
		catdb := cat.GetCategoryDB(db)
		err = db.Create(&catdb).Error
		if err != nil {
			return uuid.UUID{}, err
		}
		return catdb.ID, nil
	}
	return category.ID, nil
}

func (cdb *CategoryDB) GetCategory(db *database.Handler) (cat Category) {
	cat.ID = cdb.ID
	cat.Name = cdb.Name
	_ = json.Unmarshal(cdb.Metadata, &cat.Metadata)
	return
}
func (c *Category) GetCategoryDB(db *database.Handler) (catdb CategoryDB) {
	catdb.ID = c.ID
	catdb.Name = c.Name
	catdb.Metadata, _ = json.Marshal(c.Metadata)
	return
}
