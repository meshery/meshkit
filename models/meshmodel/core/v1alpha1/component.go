package v1alpha1

const ComponentDefinitionKindKey = "ComponentDefinition"

type TypeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

// use NewComponent function for instantiating
type Component struct {
	TypeMeta
	ComponentSpec
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// for backward compatibility
	Spec string `json:"spec,omitempty"`
}

type ComponentSpec struct {
	Schematic map[string]interface{} `json:"schematic,omitempty"`
}

func NewComponent() Component {
	comp := Component{}
	comp.APIVersion = "core.meshery.io/v1alpha1"
	comp.Kind = ComponentDefinitionKindKey
	return comp
}
