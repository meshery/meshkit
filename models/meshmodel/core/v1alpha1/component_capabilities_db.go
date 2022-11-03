package v1alpha1

import (
	"encoding/json"

	"github.com/google/uuid"
)

// This file consists of helper methods and structs that database(gorm) will use to interact with meshmodel components
type ComponentDB struct {
	TypeMeta
	ComponentSpecDB
	Metadata []byte `json:"metadata"`
	// for backward compatibility
	Spec string `json:"spec,omitempty"`
}

type ComponentSpecDB struct {
	Schematic []byte `json:"schematic,omitempty"`
}

type ComponentCapabilityDB struct {
	ID uuid.UUID `json:"id,omitempty"`
	ComponentDB
	Capability
}

// ComponentCapabilityFromCCDB produces a client facing instance of ComponentCapability from a database representation of ComponentCapability.
// Use this function to interconvert any time the ComponentCapability is fetched from the database and is to be returned to client.
func ComponentCapabilityFromCCDB(cdb ComponentCapabilityDB) (c ComponentCapability) {
	c.Capability = cdb.Capability
	c.TypeMeta = cdb.TypeMeta
	c.Spec = cdb.Spec
	m := make(map[string]interface{})
	_ = json.Unmarshal(cdb.Metadata, &m)
	c.Metadata = m
	schematic := make(map[string]interface{})
	_ = json.Unmarshal(cdb.Schematic, &schematic)
	c.Schematic = schematic
	return
}

// ComponentCapabilityDBFromCC produces a database compatible instance of ComponentCapability from a client representation of ComponentCapability.
// Use this function to interconvert any time the ComponentCapability is created by some client and is to be saved to the database.
func ComponentCapabilityDBFromCC(c ComponentCapability) (cdb ComponentCapabilityDB) {
	cdb.Capability = c.Capability
	cdb.TypeMeta = c.TypeMeta
	cdb.Spec = c.Spec
	cdb.Metadata, _ = json.Marshal(c.Metadata)
	cdb.Schematic, _ = json.Marshal(c.Schematic)
	return
}
