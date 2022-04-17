package models

import "encoding/json"

type DockerComposeFile struct {
	Version  json.Number `yaml:"version" json:"version"`
	Services interface{} `yaml:"services" json:"services"` // more constraints should be added to this type
}
