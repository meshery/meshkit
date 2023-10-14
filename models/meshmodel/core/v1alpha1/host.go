package v1alpha1

import "github.com/google/uuid"

type MeshModelHostsWithEntitySummary struct {
	ID       uuid.UUID     `json:"id"`
	Hostname string        `json:"hostname"`
	Port     int           `json:"port"`
	Summary  EntitySummary `json:"summary"`
}
type EntitySummary struct {
	Models        int64 `json:"models"`
	Components    int64 `json:"components"`
	Relationships int64 `json:"relationships"`
	Policies      int64 `json:"policies"`
}
type MesheryHostSummaryDB struct {
	HostID        uuid.UUID `json:"-" gorm:"id"`
	Hostname      string    `json:"-" gorm:"hostname"`
	Port          int       `json:"-" gorm:"port"`
	Models        int64     `json:"-" gorm:"models"`
	Components    int64     `json:"-" gorm:"components"`
	Relationships int64     `json:"-" gorm:"relationships"`
	Policies      int64     `json:"-" gorm:"policies"`
}

type HostFilter struct {
	Name        string
	Greedy      bool //when set to true - instead of an exact match, name will be prefix matched
	Trim        bool //when set to true - the schema is not returned
	DisplayName string
	Version     string
	Sort        string //asc or desc. Default behavior is asc
	OrderOn     string
	Limit       int //If 0 or  unspecified then all records are returned and limit is not used
	Offset      int
}
