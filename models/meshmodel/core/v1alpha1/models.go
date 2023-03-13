package v1alpha1

import (
	"sync"

	"github.com/google/uuid"
)

var modelCreationLock sync.Mutex //Each component/relationship will perform a check and if the model already doesn't exist, it will create a model. This lock will make sure that there are no race conditions.
type ModelFilter struct {
	Name        string
	DisplayName string //If Name is already passed, avoid passing Display name unless greedy=true, else the filter will translate to an AND returning only the models where name and display name match exactly. Ignore, if this behavior is expected.
	Greedy      bool   //when set to true - instead of an exact match, name will be prefix matched. Also an OR will be performed of name and display_name
	Version     string
	Category    string
	OrderOn     string
	Sort        string //asc or desc. Default behavior is asc
	Limit       int    //If 0 or  unspecified then all records are returned and limit is not used
	Offset      int
}

// swagger:response Model
type Model struct {
	ID          uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	DisplayName string    `json:"modelDisplayName" gorm:"modelDisplayName"`
	Category    string    `json:"category"`
	SubCategory string    `json:"subCategory" gorm:"subCategory"`
}

// Create the filter from map[string]interface{}
func (cf *ModelFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}
