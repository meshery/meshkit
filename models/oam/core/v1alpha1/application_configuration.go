package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Configuration is the structure for OAM Application Configuration
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConfigurationSpec `json:"spec,omitempty"`
}

// ConfigurationSpec is the structure for the OAM Application
// Configuration Spec
type ConfigurationSpec struct {
	Components []ConfigurationSpecComponent
}

// ConfigurationSpecComponent is the struct for OAM Application
// Configuration's spec's components
type ConfigurationSpecComponent struct {
	ComponentName string
	Traits        []ConfigurationSpecComponentTrait
	Scopes        []ConfigurationSpecComponentScope
}

// ConfigurationSpecComponentTrait is the struct
type ConfigurationSpecComponentTrait struct {
	Name       string
	Properties map[string]interface{}
}

// ConfigurationSpecComponentScope struct defines the structure
// for scope of OAM application configuration's spec's component's scope
type ConfigurationSpecComponentScope struct {
	ScopeRef ConfigurationSpecComponentScopeRef
}

// ConfigurationSpecComponentScopeRef struct defines the structure for
// scope of OAM application configuration's spec's component's scope's
// scopeRef
type ConfigurationSpecComponentScopeRef struct {
	metav1.TypeMeta `json:",inline"`
	Name            string
}
