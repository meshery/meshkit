package v1alpha1

import "github.com/google/uuid"

type MesheryHostsDisplay struct {
	ID       uuid.UUID           `json:"id"`
	Hostname string              `json:"hostname"`
	Port     int                 `json:"port"`
	Summary  HostIndividualCount `json:"summary"`
}
type HostIndividualCount struct {
	Models        int64 `json:"models"`
	Components    int64 `json:"components"`
	Relationships int64 `json:"relationships"`
	Policies      int64 `json:"policies"`
}
