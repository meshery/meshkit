package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// unmarshal returns parses the JSON config data and stores the value in the reference to result
func Unmarshal(obj string, result interface{}) error {
	obj = strings.TrimSpace(obj)
	err := json.Unmarshal([]byte(obj), result)
	if err != nil {
		if e, ok := err.(*json.SyntaxError); ok {
			return ErrUnmarshalSyntax(err, e.Offset)
		}
		if e, ok := err.(*json.UnmarshalTypeError); ok {
			return ErrUnmarshalType(err, e.Value)
		}
		if e, ok := err.(*json.UnsupportedTypeError); ok {
			return ErrUnmarshalUnsupportedType(err, e.Type)
		}
		if e, ok := err.(*json.UnsupportedValueError); ok {
			return ErrUnmarshalUnsupportedValue(err, e.Value)
		}
		if e, ok := err.(*json.InvalidUnmarshalError); ok {
			return ErrUnmarshalInvalid(err, e.Type)
		}
		return ErrUnmarshal(err)
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

// CreateFile creates a file with the given content on the given location with
// the given filename
func CreateFile(contents []byte, filename string, location string) error {
	// Create file in -rw-r--r-- mode
	fd, err := os.OpenFile(filepath.Join(location, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err = fd.Write(contents); err != nil {
		fd.Close()
		return err
	}

	if err = fd.Close(); err != nil {
		return err
	}

	return nil
}

// ReadFileSource supports "http", "https" and "file" protocols.
// it takes in the location as a uri and returns the contents of
// file as a string.
func ReadFileSource(uri string) (string, error) {
	if strings.HasPrefix(uri, "http") {
		return ReadRemoteFile(uri)
	}
	if strings.HasPrefix(uri, "file") {
		return ReadLocalFile(uri)
	}

	return "", ErrInvalidProtocol()
}

// ReadRemoteFile takes in the location of a remote file
// in the format 'http://location/of/file' or 'https://location/file'
// and returns the content of the file if the location is valid and
// no error occurs
func ReadRemoteFile(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return " ", err
	}
	if response.StatusCode == http.StatusNotFound {
		return " ", ErrRemoteFileNotFound(url)
	}

	defer response.Body.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return " ", ErrReadingRemoteFile(err)
	}

	return buf.String(), nil
}

// ReadLocalFile takes in the location of a local file
// in the format `file://location/of/file` and returns
// the content of the file if the path is valid and no
// error occurs
func ReadLocalFile(location string) (string, error) {
	// remove the protocol prefix
	location = strings.TrimPrefix(location, "file://")

	// Need to support variable file locations hence
	// #nosec
	data, err := ioutil.ReadFile(location)
	if err != nil {
		return "", ErrReadingLocalFile(err)
	}

	return string(data), nil
}
