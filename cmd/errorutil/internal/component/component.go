package component

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

// ComponentInfo specifies type, name, and minimum error code of the current component.
// Refer to the corresponding design document for valid types and names, extend if necessary.
type (
	ComponentInfo struct {
		Type         string `yaml:"type" json:"type"`                     // the type of the component, e.g. "adapter"
		Name         string `yaml:"name" json:"name"`                     // the name of the component, e.g. "kuma"
		MinErrorCode int    `yaml:"min_error_code" json:"min_error_code"` // the next error code to use. this value will be updated automatically.
	}
)

func ReadComponentInfoFile(dir string) (ComponentInfo, error) {
	info := ComponentInfo{}
	file, err := ioutil.ReadFile(filepath.Join(dir, "component_info.json"))
	if err != nil {
		return info, err
	}

	err = json.Unmarshal([]byte(file), &info)
	return info, err
}

func GetNextErrorCode() string {
	return "\"newValue\""
}
