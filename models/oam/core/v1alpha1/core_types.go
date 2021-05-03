package v1alpha1

// DefinitionRef struct describes the structure for DefinitionRef
// which are used within TraitDefinition, WorkloadDefinition, etc
type DefinitionRef struct {
	Name string `json:"name,omitempty"`
}
