package v1alpha1

import (
	"encoding/json"

	"github.com/google/uuid"
)

// This file consists of methods and structs that database(gorm) will use to interact with meshmodel components
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
	capability
}
type capabilityDB struct {
	// Host is the address of the service registering the capability
	Host string `json:"host,omitempty"`
}

func ComponentCapabilityFromCCDB(cdb ComponentCapabilityDB) (c ComponentCapability) {
	c.capability = cdb.capability
	c.TypeMeta = cdb.TypeMeta
	c.Spec = cdb.Spec
	m := make(map[string]interface{})
	json.Unmarshal(cdb.Metadata, &m)
	c.Metadata = m
	schematic := make(map[string]interface{})
	json.Unmarshal(cdb.Schematic, &schematic)
	c.Schematic = schematic
	return
}
func ComponentCapabilityDBFromCC(c ComponentCapability) (cdb ComponentCapabilityDB) {
	cdb.capability = c.capability
	cdb.TypeMeta = c.TypeMeta
	cdb.Spec = c.Spec
	cdb.Metadata, _ = json.Marshal(c.Metadata)
	cdb.Schematic, _ = json.Marshal(c.Schematic)
	return
}
