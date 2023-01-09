package v1alpha1

type ModelFilter struct {
	Name     string
	Version  string
	Category string
}

// Create the filter from map[string]interface{}
func (cf *ModelFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
	cf.Name = m["name"].(string)
}
