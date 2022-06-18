package models

type DockerComposeFile struct {
	Version  string      `yaml:"version,omitempty" json:"version,omitempty"`
	Services interface{} `yaml:"services" json:"services"` // more constraints should be added to this type
	Networks interface{} `yaml:"networks,omitempty" json:"networks,omitempty"`
	Volumes  interface{} `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Configs  interface{} `yaml:"configs,omitempty" json:"configs,omitempty"`
	Secrets  interface{} `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}
