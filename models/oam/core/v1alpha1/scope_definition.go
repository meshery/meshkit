package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScopeDefinition is the struct for OAM ScopeDefinition construct
type ScopeDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScopeDefinitionSpec
}

// ScopeDefinitionSpec is the struct for OAM ScopeDefinition's spec
type ScopeDefinitionSpec struct {
	AllowComponentOverlap bool          `json:"allowComponentOverlap,omitempty"`
	DefinitionRef         DefinitionRef `json:"definitionRef,omitempty"`
}
