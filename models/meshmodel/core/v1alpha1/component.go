package v1alpha1

const ComponentDefinitionKindKey = "ComponentDefinition"

type TypeMeta struct {
	Kind       string `json:"kind,omitempty" gorm:"kind"`
	APIVersion string `json:"apiVersion,omitempty" gorm:"apiVersion"`
}

// use NewComponent function for instantiating
type Component struct {
	TypeMeta      `gorm:"embedded"`
	ComponentSpec `gorm:"embedded"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" gorm:"type:JSONB"`
	// for backward compatibility
	Spec string `json:"spec,omitempty"`
}

type ComponentSpec struct {
	Schematic map[string]interface{} `json:"schematic,omitempty" gorm:"type:JSONB"`
}

type ComponentCapability struct {
	Component
	capability
}
type capability struct {
	ID string `json:"id,omitempty"`
	// Host is the address of the service registering the capability
	Host string `json:"host,omitempty"`
}

func NewComponent() Component {
	comp := Component{}
	comp.APIVersion = "core.meshery.io/v1alpha1"
	comp.Kind = ComponentDefinitionKindKey
	comp.Metadata = make(map[string]interface{}, 1)
	return comp
}
