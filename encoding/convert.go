package encoding

import (
	"gopkg.in/yaml.v3"
)

func ToYaml(data []byte) ([]byte, error) {
	var out map[string]interface{}
	err := Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(out)
}
