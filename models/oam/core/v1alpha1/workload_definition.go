package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkloadDefinition is the struct for OAM WorkloadDefinition construct
type WorkloadDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WorkloadDefinitionSpec `json:"spec,omitempty"`
}

// WorkloadDefinitionSpec is the struct for OAM WorkloadDefinitionSpec's spec
type WorkloadDefinitionSpec struct {
	DefinitionRef DefinitionRef `json:"definitionRef,omitempty"`
}
