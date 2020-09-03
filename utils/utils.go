package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
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

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// GetHome returns the home path
func GetHome() string {
	usr, _ := user.Current()
	return usr.HomeDir
}
