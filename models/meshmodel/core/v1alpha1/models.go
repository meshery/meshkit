package v1alpha1

type ModelFilter struct {
	Name     string
	Greedy   bool //when set to true - instead of an exact match, name will be prefix matched
	Version  string
	Category string
	Sort     bool //when set to true -  the returned entities will be sorted on Name
	Limit    int  //If 0 or  unspecified then all records are returned and limit is not used
	Offset   int
}

// Create the filter from map[string]interface{}
func (cf *ModelFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}
