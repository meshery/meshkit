package models

import "time"

type Request struct {
	Meta string      `json:"meta,omitempty"`
	Body interface{} `json:"body,omitempty"`
}

type Response struct {
	Code string      `json:"code,omitempty"`
	Body interface{} `json:"body,omitempty"`
}

type Health struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

type Stats struct {
	Name      string    `json:"name"`
	Port      string    `json:"port"`
	Proxy     string    `json:"proxy"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedat,string"`
}
