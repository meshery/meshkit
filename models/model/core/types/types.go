package types

type CapabilityType string

const (
	ComponentDefinition    CapabilityType = "component"
	PolicyDefinition       CapabilityType = "policy"
	RelationshipDefinition CapabilityType = "relationship"
	Model                  CapabilityType = "model"
)

// Each entity will have it's own Filter implementation via which it exposes the nobs and dials to fetch entities
type Filter interface {
	Create(map[string]interface{})
}
