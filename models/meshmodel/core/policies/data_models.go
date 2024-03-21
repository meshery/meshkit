package policies

// Define a simple policy struct
type RegoPolicy struct {
	Rules string `json:"rules"`
}

type RelationObject struct {
	DestinationID   string                 `json:"destination_id,omitempty"`
	DestinationName string                 `json:"destination_name,omitempty"`
	SourceId        string                 `json:"source_id,omitempty"`
	SourceName      string                 `json:"source_name,omitempty"`
	Port            map[string]interface{} `json:"port,omitempty"`
}

type NetworkPolicyRegoResponse struct {
	ServicePodRelationships        []RelationObject `json:"service_pod_relationships,omitempty"`
	ServiceDeploymentRelationships []RelationObject `json:"service_deployment_relationships,omitempty"`
}

// Add response struct based on schema for all relationships evaluations. binding, inventory, network...