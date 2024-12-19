package orchestration

import "github.com/meshery/schemas/models/v1beta1/component"

const (
	ResourceSourceDesignIdLabelKey    = "design.meshery.io/id"
	ResourceSourceDesignNameLabelKey  = "design.meshery.io/name"
	ResourceSourceComponentIdLabelKey = "component.meshery.io/id"
)

// Enriches the component with additional labels and annotations related to the design and component
// This is useful for tracking the source of the component and design in the cluster
// for allowing orchestration through Meshery
func EnrichComponentWithMesheryMetadata(comp *component.ComponentDefinition, designId string, designName string) error {

	// Initialize Configuration if nil
	if comp.Configuration == nil {
		comp.Configuration = make(map[string]interface{})
	}

	// Check and initialize Metadata if absent
	metadata, ok := comp.Configuration["metadata"].(map[string]interface{})
	if !ok || metadata == nil {
		metadata = map[string]interface{}{
			"labels":      make(map[string]interface{}),
			"annotations": make(map[string]interface{}),
		}
		comp.Configuration["metadata"] = metadata
	}

	// Check and initialize Labels if absent
	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok || labels == nil {
		labels = make(map[string]interface{})
		metadata["labels"] = labels
	}

	annotations, ok := metadata["annotations"].(map[string]interface{})

	if !ok || annotations == nil {
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	}

	// Assign the new label
	labels[ResourceSourceDesignIdLabelKey] = designId
	annotations[ResourceSourceDesignNameLabelKey] = designName
	annotations[ResourceSourceComponentIdLabelKey] = comp.Id.String()

	return nil
}
