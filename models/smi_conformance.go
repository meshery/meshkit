package models

import (
	"github.com/google/uuid"
)

type Specification struct {
	Name       string `json:"name,omitempty" db:"name"`
	Version    string `json:"version,omitempty" db:"version"`
	Assertions string `json:"assertions,omitempty" db:"assertion"`
	Duration   string `json:"duration,omitempty" db:"duration"`
	Result     string `json:"result,omitempty" db:"result"`
	Reason     string `json:"reason,omitempty" db:"reason"`
}

// SmiResult - represents the results from Meshery smi conformance test run
type SmiResult struct {
	ID                 uuid.UUID       `json:"meshery_id,omitempty" db:"id"`
	Date               string          `json:"datetime,omitempty" dc:"datetime"`
	ServiceMesh        string          `json:"servicemesh,omitempty" db:"servicemesh"`
	ServiceMeshVersion string          `json:"servicemeshversion,omitempty" db:"servicemeshversion"`
	Capability         string          `json:"capability,omitempty" db:"capability"`
	Status             string          `json:"status,omitempty" db:"status"`
	Specifications     []Specification `json:"specifications,omitempty" db:"specifications"`
}
