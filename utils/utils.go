package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/meshery/meshkit/models/meshmodel/entity"
	"gopkg.in/yaml.v3"
)

type fileWriter interface {
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Close() error
}

var (
	createWritableFile = func(path string) (fileWriter, error) {
		return os.Create(path)
	}
	openWritableFile = func(path string, flag int, perm os.FileMode) (fileWriter, error) {
		return os.OpenFile(path, flag, perm)
	}
	jsonMarshal       = json.Marshal
	jsonMarshalIndent = json.MarshalIndent
	tarHeaderForFile  = tar.FileInfoHeader
	relativePath      = filepath.Rel
	copyToTarWriter   = io.Copy
)

// transforms the keys of a Map recursively with the given transform function
func TransformMapKeys(input map[string]interface{}, transformFunc func(string) string) map[string]interface{} {
	output := make(map[string]interface{})
	for k, v := range input {
		transformedKey := transformFunc(k)
		value, ok := v.(map[string]interface{})
		if !ok {
			output[transformedKey] = v
		} else {
			output[transformedKey] = TransformMapKeys(value, transformFunc)
		}
	}
	return output
}

// Deprecated: Use Unmarshal from encoding package.
// TODO: Replace the usages from all projects.
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
	result, err := jsonMarshal(obj)
	if err != nil {
		return " ", ErrMarshal(err)
	}
	return string(result), nil
}

func Filepath() string {
	_, fn, line, _ := runtime.Caller(0)

	return fmt.Sprintf("file: %s, line: %d", fn, line)
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
	fd, err := openWritableFile(filepath.Join(location, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err = fd.Write(contents); err != nil {
		_ = fd.Close()
		return err
	}

	if err = fd.Close(); err != nil {
		return err
	}

	return nil
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
	data, err := os.ReadFile(location)
	if err != nil {
		return "", ErrReadingLocalFile(err)
	}

	return string(data), nil
}


func Contains[G []K, K comparable](slice G, ele K) bool {
	for _, item := range slice {
		if item == ele {
			return true
		}
	}
	return false
}

func FindIndexInSlice(key string, col []string) int {
	for i, n := range col {
		if n == key {
			return i
		}
	}
	return -1
}

func Cast[K any](val interface{}) (K, error) {
	var assertedValue K
	if IsInterfaceNil(val) {
		return assertedValue, ErrTypeCast(fmt.Errorf("nil interface cannot be type casted"))
	}
	var ok bool
	assertedValue, ok = val.(K)
	if !ok {
		return assertedValue, ErrTypeCast(fmt.Errorf("the underlying type of the interface is %s", reflect.TypeOf(val).Name()))
	}
	return assertedValue, nil
}

func MarshalAndUnmarshal[fromType any, toType any](val fromType) (unmarshalledvalue toType, err error) {
	data, err := Marshal(val)
	if err != nil {
		return
	}

	err = Unmarshal(data, &unmarshalledvalue)
	if err != nil {
		return
	}
	return
}

func IsClosed[K any](ch chan K) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// WriteToFile writes the given content to the given file path
func WriteToFile(path string, content string) error {
	file, err := createWritableFile(path)
	if err != nil {
		return ErrCreateFile(err, path)
	}

	_, err = file.WriteString(content)
	if err != nil {
		return ErrWriteFile(err, path)
	}
	// Close the file to save the changes.
	err = file.Close()
	if err != nil {
		return ErrWriteFile(err, path)
	}
	return nil
}

// FormatName formats the given string to by replacing " " with "-"
func FormatName(input string) string {
	formatedName := strings.ReplaceAll(input, " ", "-")
	formatedName = strings.ToLower(formatedName)
	return formatedName
}

func GetRandomAlphabetsOfDigit(length int) (s string) {
	charSet := "abcdedfghijklmnopqrstuvwxyz"
	for i := 0; i < length; i++ {
		random := mathrand.Intn(len(charSet))
		randomChar := charSet[random]
		s += string(randomChar)
	}
	return
}

// combineErrors merges a slice of error
// into one error separated by the given separator
func CombineErrors(errs []error, sep string) error {
	if len(errs) == 0 {
		return nil
	}

	var errString []string
	for _, err := range errs {
		errString = append(errString, err.Error())
	}

	return errors.New(strings.Join(errString, sep))
}

func MergeMaps(mergeInto, toMerge map[string]interface{}) map[string]interface{} {
	if mergeInto == nil {
		mergeInto = make(map[string]interface{})
	}
	for k, v := range toMerge {
		mergeInto[k] = v
	}
	return mergeInto
}

func WriteYamlToFile[K any](outputPath string, data K) (err error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return ErrCreateFile(err, outputPath)
	}
	defer func() { _ = file.Close() }()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	defer func() { _ = encoder.Close() }()
	defer func() {
		if recovered := recover(); recovered != nil {
			switch value := recovered.(type) {
			case error:
				err = ErrMarshal(value)
			default:
				err = ErrMarshal(fmt.Errorf("%v", value))
			}
		}
	}()

	if err := encoder.Encode(data); err != nil {
		return ErrMarshal(err)
	}

	return nil
}

func WriteJSONToFile[K any](outputPath string, data K) error {
	byt, err := jsonMarshalIndent(data, "", "  ")

	if err != nil {
		return ErrMarshal(err)
	}
	err = os.WriteFile(outputPath, byt, 0644)

	if err != nil {
		return ErrWriteFile(err, outputPath)
	}
	return nil
}

func CreateDirectory(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		err = ErrCreateDir(err, path)

		return err
	}
	return nil
}

func ReplaceSpacesAndConvertToLowercase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", ""))
}
func ReplaceSpacesWithHyphenAndConvertToLowercase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}
func ExtractDomainFromURL(location string) string {
	parsedURL, err := url.Parse(location)
	// If unable to extract domain return the location as is.
	if err != nil {
		return location
	}
	return regexp.MustCompile(`(([a-zA-Z0-9]+\.)([a-zA-Z0-9]+))$`).FindString(parsedURL.Hostname())
}

func IsInterfaceNil(val interface{}) bool {
	if val == nil {
		return true
	}
	return reflect.ValueOf(val).IsZero()
}

func IsSchemaEmpty(schema string) (valid bool) {
	if schema == "" {
		return
	}
	m := make(map[string]interface{})
	_ = json.Unmarshal([]byte(schema), &m)
	if m["properties"] == nil {
		return
	}
	valid = true
	return
}
func FindEntityType(content []byte) (entity.EntityType, error) {
	var tempMap map[string]interface{}
	if err := json.Unmarshal(content, &tempMap); err != nil {
		return "", ErrUnmarshal(err)
	}
	schemaVersion, err := Cast[string](tempMap["schemaVersion"])
	if err != nil {
		return "", ErrInvalidSchemaVersion
	}
	lastIndex := strings.LastIndex(schemaVersion, "/")
	if lastIndex != -1 {
		schemaVersion = schemaVersion[:lastIndex]
	}
	switch schemaVersion {
	case "relationships.meshery.io":
		return entity.RelationshipDefinition, nil
	case "components.meshery.io":
		return entity.ComponentDefinition, nil
	case "models.meshery.io":
		return entity.Model, nil
	case "policies.meshery.io":
		return entity.PolicyDefinition, nil
	}
	return "", ErrInvalidSchemaVersion
}

// RecursiveCastMapStringInterfaceToMapStringInterface will convert a
// map[string]interface{} recursively => map[string]interface{}
func RecursiveCastMapStringInterfaceToMapStringInterface(in map[string]interface{}) map[string]interface{} {
	res := ConvertMapInterfaceMapString(in)
	out, _ := res.(map[string]interface{})

	return out
}

// ConvertMapInterfaceMapString converts map[interface{}]interface{} => map[string]interface{}
//
// It will also convert []interface{} => []string
func ConvertMapInterfaceMapString(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, v2 := range x {
			switch k2 := k.(type) {
			case string:
				m[k2] = ConvertMapInterfaceMapString(v2)
			default:
				m[fmt.Sprint(k)] = ConvertMapInterfaceMapString(v2)
			}
		}
		v = m

	case []interface{}:
		for i, v2 := range x {
			x[i] = ConvertMapInterfaceMapString(v2)
		}

	case map[string]interface{}:
		for k, v2 := range x {
			x[k] = ConvertMapInterfaceMapString(v2)
		}
	}

	return v
}
func ConvertToJSONCompatible(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for key, value := range v {
			m[key.(string)] = ConvertToJSONCompatible(value)
		}
		return m
	case []interface{}:
		for i, item := range v {
			v[i] = ConvertToJSONCompatible(item)
		}
	}
	return data
}
func YAMLToJSON(content []byte) ([]byte, error) {
	var jsonData interface{}
	if err := yaml.Unmarshal(content, &jsonData); err == nil {
		jsonData = ConvertToJSONCompatible(jsonData)
		convertedContent, err := jsonMarshal(jsonData)
		if err == nil {
			content = convertedContent
		} else {
			return nil, ErrUnmarshal(err)
		}
	} else {
		return nil, ErrUnmarshal(err)
	}
	return content, nil
}
// Convert path to svg Data
func ReadSVGData(baseDir, path string) (string, error) {
	fullPath := baseDir + path
	svgData, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(svgData), nil
}
func Compress(src string, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	defer func() { _ = zr.Close() }()
	tw := tar.NewWriter(zr)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tarHeaderForFile(fi, file)
		if err != nil {
			return err
		}

		relPath, err := relativePath(src, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}

			_, err = copyToTarWriter(tw, data)
			_ = data.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Check if a string is purely numeric
func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Split version into components (numeric and non-numeric) using both '.' and '-'
func splitVersion(version string) []string {
	version = strings.ReplaceAll(version, "-", ".")
	return strings.Split(version, ".")
}

// Compare two version strings
func compareVersions(v1, v2 string) int {
	v1Components := splitVersion(v1)
	v2Components := splitVersion(v2)

	maxLen := len(v1Components)
	if len(v2Components) > maxLen {
		maxLen = len(v2Components)
	}

	for i := 0; i < maxLen; i++ {
		var part1, part2 string
		if i < len(v1Components) {
			part1 = v1Components[i]
		}
		if i < len(v2Components) {
			part2 = v2Components[i]
		}

		if isNumeric(part1) && isNumeric(part2) {
			num1, _ := strconv.Atoi(part1)
			num2, _ := strconv.Atoi(part2)
			if num1 != num2 {
				return num1 - num2
			}
		} else if isNumeric(part1) && !isNumeric(part2) {
			return -1
		} else if !isNumeric(part1) && isNumeric(part2) {
			return 1
		} else {
			if part1 != part2 {
				return strings.Compare(part1, part2)
			}
		}
	}

	return 0
}

// Function to get all version directories sorted in descending order
func GetAllVersionDirsSortedDesc(modelVersionsDirPath string) ([]string, error) {
	type versionInfo struct {
		original string
		dirPath  string
	}
	entries, err := os.ReadDir(modelVersionsDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory '%s': %w", modelVersionsDirPath, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no version directories found in '%s'", modelVersionsDirPath)
	}

	versions := []versionInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		versionDirPath := filepath.Join(modelVersionsDirPath, entry.Name())
		versionStr := entry.Name()

		// Optionally remove leading 'v'
		versionStr = strings.TrimPrefix(versionStr, "v")

		if versionStr == "" {
			continue
		}

		versions = append(versions, versionInfo{
			original: versionStr,
			dirPath:  versionDirPath,
		})
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no valid version directories found in '%s'", modelVersionsDirPath)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].original, versions[j].original) > 0
	})

	sortedDirPaths := make([]string, len(versions))
	for i, v := range versions {
		sortedDirPaths[i] = v.dirPath
	}

	return sortedDirPaths, nil
}

// isDirectoryNonEmpty checks if a directory exists and is non-empty
func IsDirectoryNonEmpty(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	if !fi.IsDir() {
		return false
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	return len(entries) > 0
}


// TruncateErrorMessage truncates an error message to a specified word limit.
func TruncateErrorMessage(err error, wordLimit int) error {
	if err == nil {
		return nil
	}

	words := strings.Fields(err.Error())
	if len(words) > wordLimit {
		words = words[:wordLimit]
		return errors.New(strings.Join(words, " ") + "...")
	}

	return err
}
