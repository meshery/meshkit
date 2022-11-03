package v1alpha1

const ComponentDefinitionKindKey = "ComponentDefinition"

type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion"`
}

// use NewComponent function for instantiating
type Component struct {
	TypeMeta      `gorm:"embedded" yaml:"typemeta"`
	ComponentSpec `gorm:"embedded" yaml:"componentspec"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" yaml:"metadata"`
	// for backward compatibility
	Spec string `json:"spec,omitempty" yaml:"spec"`
}
type Capability struct {
	// Host is the address of the service registering the capability
	Host string `json:"host,omitempty" yaml:"host"`
}
type ComponentSpec struct {
	Schematic map[string]interface{} `json:"schematic,omitempty" yaml:"schematic"`
}

func NewComponent() Component {
	comp := Component{}
	comp.APIVersion = "core.meshery.io/v1alpha1"
	comp.Kind = ComponentDefinitionKindKey
	comp.Metadata = make(map[string]interface{}, 1)
	return comp
}

type ComponentCapability struct {
	Component  `yaml:"component"`
	Capability `yaml:"capability"`
}
