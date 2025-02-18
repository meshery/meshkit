package v1beta1

import (
	"gorm.io/gorm/clause"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/entity"
	"github.com/layer5io/meshkit/models/meshmodel/registry"
	"github.com/meshery/schemas/models/v1beta1/subCategory"
)

type SubCategoryFilter struct {
	Id      string
	Name    string
	OrderOn string
	Greedy  bool
	Sort    string //asc or desc. Default behavior is asc
	Limit   int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset  int
}

// Create the filter from map[string]interface{}
func (cf *SubCategoryFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}

func (cf *SubCategoryFilter) GetById(db *database.Handler) (entity.Entity, error) {
	c := &subCategory.SubCategoryDefinition{}
	err := db.First(c, "id = ?", cf.Id).Error
	if err != nil {
		return nil, registry.ErrGetById(err, cf.Id)
	}
	return c, err
}

func (cf *SubCategoryFilter) Get(db *database.Handler) ([]entity.Entity, int64, int, error) {
	var scatdb []subCategory.SubCategoryDefinition
	var scat []entity.Entity
	finder := db.Model(&scatdb).Debug()

	// total count before pagination
	var count int64

	if cf.Name != "" {
		if cf.Greedy {
			finder = finder.Where("name LIKE ?", "%"+cf.Name+"%")
		} else {
			finder = finder.Where("name = ?", cf.Name)
		}
	}
	if cf.Id != "" {
		finder = finder.Where("id = ?", cf.Id)
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

	err := finder.Find(&scatdb).Error
	if err != nil {
		return scat, count, int(count), nil
	}

	for _, c := range scatdb {
		// resolve for loop scope
		_c := c

		scat = append(scat, &_c)
	}
	// duplicate sub_category ?
	return scat, count, int(count), nil
}
