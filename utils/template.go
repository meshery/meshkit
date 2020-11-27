package utils

import (
	"bytes"
	"html/template"
)

// MergeToTemplate merges data into the template tpl and returns the result.
func MergeToTemplate(tpl []byte, data interface{}) ([]byte, error) {
	t, err := template.New("template").Parse(bytes.NewBuffer(tpl).String())
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBufferString("")
	err = t.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
