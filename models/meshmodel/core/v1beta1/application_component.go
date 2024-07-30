package v1beta1

// // ComponentParameter is the structure for the core Application Component
// // Paramater
// type ComponentParameter struct {
// 	Name        string   `json:"name"`
// 	FieldPaths  []string `json:"fieldPaths"`
// 	Required    *bool    `json:"required,omitempty"`
// 	Description *string  `json:"description,omitempty"`
// }

// func GetAPIVersionFromComponent(comp v1beta1.ComponentDefinition) string {
// 	return comp.Annotations[MesheryAnnotationPrefix+".k8s.APIVersion"]
// }

// func GetKindFromComponent(comp v1beta1.ComponentDefinition) string {
// 	kind := strings.TrimPrefix(comp.Annotations[MesheryAnnotationPrefix+".k8s.Kind"], "/")
// 	return kind
// }

// func GetAnnotationsForWorkload(w v1beta1.ComponentDefinition) map[string]string {
// 	res := map[string]string{}

// 	for key, val := range w.Metadata {
// 		if v, ok := val.(string); ok {
// 			res[strings.ReplaceAll(fmt.Sprintf("%s.%s", MesheryAnnotationPrefix, key), " ", "")] = v
// 		}
// 	}
// 	sourceURI, ok := w.Model.Metadata["source_uri"].(string)
// 	if ok {
// 		res[fmt.Sprintf("%s.model.source_uri", MesheryAnnotationPrefix)] = sourceURI
// 	}
// 	res[fmt.Sprintf("%s.model.name", MesheryAnnotationPrefix)] = w.Model.Name
// 	res[fmt.Sprintf("%s.k8s.APIVersion", MesheryAnnotationPrefix)] = w.Component.Version
// 	res[fmt.Sprintf("%s.k8s.Kind", MesheryAnnotationPrefix)] = w.Component.Kind
// 	res[fmt.Sprintf("%s.model.version", MesheryAnnotationPrefix)] = w.Model.Version
// 	res[fmt.Sprintf("%s.model.category", MesheryAnnotationPrefix)] = w.Model.Category.Name
// 	return res
// }
