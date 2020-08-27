package utils

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// unmarshal returns parses the JSON config data and stores the value in the reference to result
func Unmarshal(obj string, result interface{}) error {

	obj = strings.TrimSpace(obj)
	err := json.Unmarshal([]byte(obj), result)
	if err != nil {
		return ErrUnmarshal(obj, err)
	}
	return nil
}

// getBool function returns the boolean config data
func GetBool(key string) (bool, error) {

	enabled, err := strconv.ParseBool(key)
	if err != nil {
		return false, ErrGetBool(key, err)
	}

	return enabled, nil
}

func StrConcat(s ...string) string {
	var buf strings.Builder

	for _, str := range s {
		buf.WriteString(str)
	}
	return buf.String()
}

func Marshal(obj interface{}) (string, error) {
	result, err := json.Marshal(obj)
	if err != nil {
		return " ", ErrMarshal(err)
	}
	return string(result), nil
}

func Filepath() string {
	_, fn, line, _ := runtime.Caller(0)

	return fmt.Sprintf("file: %s, line: %d", fn, line)
}
