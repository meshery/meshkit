package models

type DockerComposeFile struct {
	Version  string      `yaml:"version" json:"version"`
	Services interface{} `yaml:"services" json:"services"` // more constraints should be added to this type
}
