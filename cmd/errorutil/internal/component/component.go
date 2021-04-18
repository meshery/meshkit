package component

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"
)

const (
	filename = "component_info.json"
)

// Info specifies type, name, and the next error code of the current component.
// Refer to the corresponding design document for valid types and names, extend if necessary.
type Info struct {
	Name          string `yaml:"name" json:"name"`                       // the name of the component, e.g. "kuma"
	Type          string `yaml:"type" json:"type"`                       // the type of the component, e.g. "adapter"
	NextErrorCode int    `yaml:"next_error_code" json:"next_error_code"` // the next error code to use. this value will be updated automatically.
	dir           string
}

type Component interface {
	GetNextErrorCode() string
	Write() error
}

// New reads the file component_info.json from dir and returns an info struct
func New(dir string) (*Info, error) {
	info := Info{}
	file, err := ioutil.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return &info, err
	}

	err = json.Unmarshal([]byte(file), &info)
	return &info, err
}

// GetNextErrorCode returns the next error code (an int) as a string, and increments to the next error code.
func (i *Info) GetNextErrorCode() string {
	s := strconv.Itoa(i.NextErrorCode)
	i.NextErrorCode = i.NextErrorCode + 1
	return s
}

// Write writes the component info back to file.
func (i *Info) Write() error {
	jsn, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	fname := filepath.Join(i.dir, filename)
	return ioutil.WriteFile(fname, jsn, 0600)
}
