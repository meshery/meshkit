package v1alpha1

const ComponentDefinitionKindKey = "ComponentDefinition"

type TypeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type Component struct {
	TypeMeta      `json:",inline"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ComponentSpec `json:"spec,omitempty"`
}

type ComponentSpec struct {
	Schematic map[string]interface{} `json:"schematic,omitempty"`
}
