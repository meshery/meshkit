package v1beta1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Component is the structure for the core OAM Application Component
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentSpec `json:"spec,omitempty"`
}

// ComponentSpec is the structure for the core OAM Application Component Spec
type ComponentSpec struct {
	Type       string                 `json:"type"`
	Version    string                 `json:"version"`
	APIVersion string                 `json:"apiVersion"`
	Model      string                 `json:"model"`
	Settings   map[string]interface{} `json:"settings"`
	Parameters []ComponentParameter   `json:"parameters"`
}

// ComponentParameter is the structure for the core OAM Application Component
// Paramater
type ComponentParameter struct {
	Name        string   `json:"name"`
	FieldPaths  []string `json:"fieldPaths"`
	Required    *bool    `json:"required,omitempty"`
	Description *string  `json:"description,omitempty"`
}

const MesheryAnnotationPrefix = "design.meshmodel.io"

func GetAPIVersionFromComponent(comp Component) string {
	return comp.Annotations[MesheryAnnotationPrefix+".k8s.APIVersion"]
}

func GetKindFromComponent(comp Component) string {
	kind := strings.TrimPrefix(comp.Annotations[MesheryAnnotationPrefix+".k8s.Kind"], "/")
	return kind
}

func GetAnnotationsForWorkload(w ComponentDefinition) map[string]string {
	res := map[string]string{}

	for key, val := range w.Metadata {
		if v, ok := val.(string); ok {
			res[strings.ReplaceAll(fmt.Sprintf("%s.%s", MesheryAnnotationPrefix, key), " ", "")] = v
		}
	}
	sourceURI, ok := w.Model.Metadata["source_uri"].(string)
	if ok {
		res[fmt.Sprintf("%s.model.source_uri", MesheryAnnotationPrefix)] = sourceURI
	}
	res[fmt.Sprintf("%s.model.name", MesheryAnnotationPrefix)] = w.Model.Name
	res[fmt.Sprintf("%s.k8s.APIVersion", MesheryAnnotationPrefix)] = w.Component.Version
	res[fmt.Sprintf("%s.k8s.Kind", MesheryAnnotationPrefix)] = w.Component.Kind
	res[fmt.Sprintf("%s.model.version", MesheryAnnotationPrefix)] = w.Model.Version
	res[fmt.Sprintf("%s.model.category", MesheryAnnotationPrefix)] = w.Model.Category.Name
	return res
}
