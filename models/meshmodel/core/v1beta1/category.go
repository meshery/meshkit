package v1beta1

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var categoryCreationLock sync.Mutex //Each model will perform a check and if the category already doesn't exist, it will create a category. This lock will make sure that there are no race conditions.

// swagger:response Category
type Category struct {
	ID       uuid.UUID              `json:"-" yaml:"-"`
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
	Greedy  bool
	Sort    string //asc or desc. Default behavior is asc
	Limit   int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset  int
}

// "Uncategorized" is assigned when Category is empty in the component definitions.
const DefaultCategory = "Uncategorized"

// Create the filter from map[string]interface{}
func (cf *CategoryFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func (cf *CategoryFilter) Get(db *database.Handler) ([]Category, int64, int, error) {
	var catdb []CategoryDB
	var cat []Category
	finder := db.Model(&catdb)

	// total count before pagination
	var count int64

	if cf.Name != "" {
		if cf.Greedy {
			finder = finder.Where("name LIKE ?", "%"+cf.Name+"%")
		} else {
			finder = finder.Where("name = ?", cf.Name)
		}
	}
	if cf.OrderOn != "" {
		if cf.Sort == "desc" {
			finder = finder.Order(clause.OrderByColumn{Column: clause.Column{Name: cf.OrderOn}, Desc: true})
		} else {
			finder = finder.Order(cf.OrderOn)
		}
	}

	finder.Count(&count)

	if cf.Limit != 0 {
		finder = finder.Limit(cf.Limit)
	}
	if cf.Offset != 0 {
		finder = finder.Offset(cf.Offset)
	}

	if count == 0 {
		finder.Count(&count)
	}

	_ = finder.Find(&catdb).Error
	for _, c := range catdb {
		cat = append(cat, c.GetCategory(db))
	}
	// duplicate category ?
	return cat, count, int(count), nil
}

func (cat *Category) Create(db *database.Handler) (uuid.UUID, error) {
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

func (m *Category) UpdateStatus(db database.Handler, status entity.EntityStatus) error {
	return nil
}

func (c *Category) GetCategoryDB(db *database.Handler) (catdb CategoryDB) {
	catdb.ID = c.ID
	catdb.Name = c.Name
	catdb.Metadata, _ = json.Marshal(c.Metadata)
	return
}

func (cdb *CategoryDB) GetCategory(db *database.Handler) (cat Category) {
	cat.ID = cdb.ID
	cat.Name = cdb.Name
	_ = json.Unmarshal(cdb.Metadata, &cat.Metadata)
	return
}
