package v1beta1

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"gorm.io/gorm/clause"
)

type CategoryFilter struct {
    Id      string
	Name    string
	OrderOn string
	Greedy  bool
	Sort    string //asc or desc. Default behavior is asc
	Limit   int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset  int
}

// Create the filter from map[string]interface{}
func (cf *CategoryFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func (cf *CategoryFilter) GetById(db *database.Handler) (entity.Entity, error) {
    c := &v1beta1.Category{}
    err := db.First(c, "id = ?", cf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, cf.Id)
	}
    return  c, err
}

func (cf *CategoryFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	var catdb []v1beta1.Category
	var cat []entity.Entity
	finder := db.Model(&catdb).Debug()

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

	err := finder.Find(&catdb).Error
	if err != nil {
		return cat, count, int(count), nil
	}

	for _, c := range catdb {
		// resolve for loop scope
		_c := c

		cat = append(cat, &_c)
	}
	// duplicate category ?
	return cat, count, int(count), nil
}
