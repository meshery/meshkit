package v1alpha1

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/layer5io/meshkit/database"
	"github.com/layer5io/meshkit/models/meshmodel/core/types"
)

// https://docs.google.com/drawings/d/1_qzQ_YxvCWPYrOBcdqGMlMwfbsZx96SBuIkbn8TfKhU/edit?pli=1
// see RELATIONSHIPDEFINITIONS table in the diagram
// TODO: Add support for Model
type RelationshipDefinition struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Metadata map[string]interface{} `json:"metadata" yaml:"metadata"`
	// using RelType since there is a method called `Type`
	SubType   string                 `json:"subType" yaml:"subType" gorm:"subType"`
	Selectors map[string]interface{} `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time              `json:"-"`
	UpdatedAt time.Time              `json:"-"`
}

type RelationshipDefinitionDB struct {
	ID uuid.UUID `json:"-"`
	TypeMeta
	Metadata  []byte    `json:"metadata" yaml:"metadata"`
	SubType   string    `json:"subType" yaml:"subType"`
	Selectors []byte    `json:"selectors" yaml:"selectors"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// For now, only filtering by Kind, Type or SubType can be done.
// In the future, we will add support to query using `selectors` (using CUE)
// TODO: Add support for Model
type RelationshipFilter struct {
	Kind    string
	SubType string
}

// Create the filter from map[string]interface{}
func (rf *RelationshipFilter) Create(m map[string]interface{}) {
	if m == nil {
		return
	}
}

func GetRelationships(db *database.Handler, f RelationshipFilter) (rs []RelationshipDefinition) {
	var rdb []RelationshipDefinitionDB
	if f.Kind != "" {
		_ = db.Where("kind = ?", f.Kind).Find(&rdb).Error
		for _, reldb := range rdb {
			rel := reldb.GetRelationshipDefinition()
			rs = append(rs, rel)
		}
	}
	if f.SubType != "" {
		if len(rs) == 0 {
			_ = db.Where("subtype = ?", f.SubType).Find(&rdb).Error
			for _, reldb := range rdb {
				rel := reldb.GetRelationshipDefinition()
				rs = append(rs, rel)
			}
		} else {
			filteredRs := []RelationshipDefinition{}
			for _, rd := range rs {
				if rd.SubType == f.SubType {
					filteredRs = append(filteredRs, rd)
				}
			}
			rs = filteredRs
		}
	}
	if len(rs) == 0 {
		db.Find(&rdb)
		for _, reldb := range rdb {
			rel := reldb.GetRelationshipDefinition()
			rs = append(rs, rel)
		}
	}
	return
}

func (rdb *RelationshipDefinitionDB) GetRelationshipDefinition() (r RelationshipDefinition) {
	r.ID = rdb.ID
	r.TypeMeta = rdb.TypeMeta
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Metadata, &r.Metadata)
	if r.Selectors == nil {
		r.Selectors = make(map[string]interface{})
	}
	_ = json.Unmarshal(rdb.Selectors, &r.Selectors)
	r.SubType = rdb.SubType
	r.Kind = rdb.Kind
	return
}

func (r RelationshipDefinition) Type() types.CapabilityType {
	return types.RelationshipDefinition
}

func CreateRelationship(db *database.Handler, r RelationshipDefinition) (uuid.UUID, error) {
	r.ID = uuid.New()
	rdb := r.GetRelationshipDefinitionDB()
	err := db.Create(&rdb).Error
	if err != nil {
		return uuid.UUID{}, err
	}
	return r.ID, err
}

func (r *RelationshipDefinition) GetRelationshipDefinitionDB() (rdb RelationshipDefinitionDB) {
	rdb.ID = r.ID
	rdb.TypeMeta = r.TypeMeta
	rdb.Metadata, _ = json.Marshal(r.Metadata)
	rdb.Selectors, _ = json.Marshal(r.Selectors)
	rdb.Kind = r.Kind
	rdb.SubType = r.SubType
	return
}
