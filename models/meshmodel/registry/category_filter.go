package registry

import (
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1beta1"
	"gorm.io/gorm/clause"
)

type CategoryFilter struct {
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

func (cf *CategoryFilter) Get(db *database.Handler) ([]v1beta1.Category, int64, int, error) {
	var catdb []v1beta1.CategoryDB
	var cat []v1beta1.Category
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
