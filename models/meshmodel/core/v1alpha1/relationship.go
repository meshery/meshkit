package v1alpha1

import (
	"time"

	"github.com/google/uuid"
)

// https://docs.google.com/drawings/d/1_qzQ_YxvCWPYrOBcdqGMlMwfbsZx96SBuIkbn8TfKhU/edit?pli=1
// see RELATIONSHIPDEFINITIONS table in the diagram
type RelationshipDefinition struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Metadata  map[string]interface{} `json:"metadata" yaml:"metadata"`
	Type      string                 `json:"type" yaml:"type"`
	SubType   string                 `json:"subType" yaml:"subType"`
	Selectors string                 `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time              `json:"-"`
	UpdatedAt time.Time              `json:"-"`
}
