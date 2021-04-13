package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

// ErrorExport is used to export Error for e.g. documentation purposes.
//
// Type Error (errors/types.go) is not reused in order to avoid tight coupling between code and documentation of errors, e.g. on Meshery website.
// It is good practice not to use internal data types in integrations; one should in general transform between internal and external models.
// DDD calls this anti-corruption layer.
// One reason is that one might like to have a different representation externally, e.g. severity 'info' instead of '1'.
// Another one is that it is often desirable to be able to change the internal representation without the need for the consumer
// (in this case, the meshery doc) to have to adjust quickly in order to be able to handle updated content.
// The lifecycles of producers and consumers should not be tightly coupled.
type (
	ErrorExport struct {
		Name                 string `yaml:"name" json:"name"`                                   // the name of the error code variable, e.g. "ErrInstallMesh", not guaranteed to be unique as it is package scoped
		Code                 string `yaml:"code" json:"code"`                                   // the code, an int, but exported as string, e.g. "1001", guaranteed to be unique per component-type:component-name
		Severity             string `yaml:"severity" json:"severity"`                           // a textual representation of the type Severity (errors/types.go), i.e. "none", "alert", etc
		LongDescription      string `yaml:"long_description" json:"long_description"`           // might contain newlines (JSON encoded)
		ShortDescription     string `yaml:"short_description" json:"short_description"`         // might contain newlines (JSON encoded)
		ProbableCause        string `yaml:"probable_cause" json:"probable_cause"`               // might contain newlines (JSON encoded)
		SuggestedRemediation string `yaml:"suggested_remediation" json:"suggested_remediation"` // might contain newlines (JSON encoded)
	}
)

// ErrorsExport is used to export all Errors including information about the component for e.g. documentation purposes.
type ErrorsExport struct {
	ComponentName string                 `yaml:"component_name" json:"component_name"` // component type, e.g. "adapter"
	ComponentType string                 `yaml:"component_type" json:"component_type"` // component name, e.g. "kuma"
	Errors        map[string]ErrorExport `yaml:"errors" json:"errors"`                 // map of all errors with key = code
}

type ErrorInfo struct {
	Name          string `yaml:"name" json:"name"`
	Code          string `yaml:"code" json:"code"`
	CodeIsLiteral bool   `yaml:"codeIsLiteral" json:"codeIsLiteral"`
	CodeIsInt     bool   `yaml:"codeIsInt" json:"codeIsInt"`
	Path          string `yaml:"path" json:"path"`
}

type Errors struct {
	Entries       []ErrorInfo            `yaml:"entries" json:"entries"`
	LiteralCodes  map[string][]ErrorInfo `yaml:"literalCodes" json:"literalCodes"`
	CallExprCodes []ErrorInfo            `yaml:"callExprCodes" json:"callExprCodes"`
}

type Summary struct {
	MinCode    int                 `yaml:"minCode" json:"minCode"`
	MaxCode    int                 `yaml:"maxCode" json:"maxCode"`
	Duplicates map[string][]string `yaml:"duplicates" json:"duplicates"`
	IntCodes   []int               `yaml:"intCodes" json:"intCodes"`
}

func NewErrors() *Errors {
	return &Errors{LiteralCodes: make(map[string][]ErrorInfo)}
}

var errors = NewErrors()

const (
	app     = "errorutil"
	logFile = app + ".log"
)

func readComponentInfoFile() (ComponentInfo, error) {
	info := ComponentInfo{}
	file, err := ioutil.ReadFile("component_info.json")
	if err != nil {
		return info, err
	}

	err = json.Unmarshal([]byte(file), &info)
	return info, err
}

func summarizeResults(errors *Errors) *Summary {
	maxInt := int(^uint(0) >> 1)
	// TODO: need also error variables with call expressions
	summary := &Summary{MinCode: maxInt, MaxCode: -maxInt - 1, Duplicates: make(map[string][]string)}
	for k, v := range errors.LiteralCodes {
		if len(v) > 1 {
			_, ok := summary.Duplicates[k]
			if !ok {
				summary.Duplicates[k] = []string{}
			}
		}
		for _, e := range v {
			if e.CodeIsInt {
				i, _ := strconv.Atoi(e.Code)
				if i < summary.MinCode {
					summary.MinCode = i
				}
				if i > summary.MaxCode {
					summary.MaxCode = i
				}
				summary.IntCodes = append(summary.IntCodes, i)
			}
			if _, ok := summary.Duplicates[k]; ok {
				summary.Duplicates[k] = append(summary.Duplicates[k], e.Name)
			}
		}
	}
	return summary
}

func configLoging(verbose bool) {
	log.SetFormatter(&log.TextFormatter{})
	//logrus.SetFormatter(&logrus.JSONFormatter{})
	if verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func isErrorsGoFile(path string) bool {
	_, file := filepath.Split(path)
	return file == "error.go"
}

func isErrorCodeName(name string) bool {
	matched, _ := regexp.MatchString("^Err[A-Z]", name)
	return matched
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func analyze(path string) error {
	logger := log.WithFields(log.Fields{"path": path})
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			logger.WithFields(log.Fields{"decl": "func", "name": decl.Name.String()}).Debug("ast declaration")
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					logger.WithFields(log.Fields{"decl": "importspec", "name": strings.Trim(spec.Path.Value, "\"")}).Debug("ast declaration")
				case *ast.TypeSpec:
					logger.WithFields(log.Fields{"decl": "typespec", "name": spec.Name.String()}).Debug("ast declaration")
				case *ast.ValueSpec:
					for _, id := range spec.Names {
						if isErrorCodeName(id.Name) {
							value0 := id.Obj.Decl.(*ast.ValueSpec).Values[0]
							isLiteral := false
							codeValue := ""
							switch value := value0.(type) {
							case *ast.BasicLit:
								isLiteral = true
								codeValue = strings.Trim(value.Value, "\"")
								logger.WithFields(log.Fields{"name": id.Name, "value": codeValue}).Info("Err* variable detected with literal value.")
							case *ast.CallExpr:
								logger.WithFields(log.Fields{"name": id.Name}).Warn("Err* variable detected with call expression value.")
							}
							ec := &ErrorInfo{
								Name:          id.Name,
								Code:          codeValue,
								CodeIsLiteral: isLiteral,
								CodeIsInt:     isInt(codeValue),
								Path:          path,
							}
							errors.Entries = append(errors.Entries, *ec)
							if isLiteral {
								key := codeValue
								if codeValue == "" {
									key = "no_code"
								}
								_, ok := errors.LiteralCodes[key]
								if !ok {
									errors.LiteralCodes[key] = []ErrorInfo{}
								}
								errors.LiteralCodes[key] = append(errors.LiteralCodes[key], *ec)
							} else {
								errors.CallExprCodes = append(errors.CallExprCodes, *ec)
							}
						}
					}
				default:
					logger.Debug(fmt.Sprintf("unhandled token type: %s", decl.Tok))
				}
			}
		default:
			logger.Debug(fmt.Sprintf("unhandled declaration: %v at %v", decl, decl.Pos()))
		}
	}
	return nil
}

func walk() {
	rootDir := "."
	subDirsToSkip := []string{".git", ".github"}
	log.Info(fmt.Sprintf("root directory: %s", rootDir))
	log.Info(fmt.Sprintf("subdirs to skip: %v", subDirsToSkip))

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		logger := log.WithFields(log.Fields{"path": path})
		if err != nil {
			logger.WithFields(log.Fields{"error": fmt.Sprintf("%v", err)}).Warn("failure accessing path")
			return err
		}
		if info.IsDir() && contains(subDirsToSkip, info.Name()) {
			logger.Debug("skipping dir")
			return filepath.SkipDir
		}
		if info.IsDir() {
			logger.Debug("handling dir")
		} else {
			if filepath.Ext(path) == ".go" {
				isErrorsGoFile := isErrorsGoFile(path)
				logger.WithFields(log.Fields{"iserrorsfile": fmt.Sprintf("%v", isErrorsGoFile)}).Debug("handling Go file")
				err := analyze(path)
				if err != nil {
					logger.Errorf("error on analyze: %v", err)
				}
			} else {
				logger.Debug("skipping file")
			}
		}
		return nil
	})
	if err != nil {
		log.Error(fmt.Sprintf("error walking the path %q: %v\n", rootDir, err))
		return
	}
}

func main() {
	logFile, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.TextFormatter{})

	info, err := readComponentInfoFile()
	if err != nil {
		log.Fatalf("Unable to read component info file (%v)", err)
		return
	}

	var cmdAnalyze = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze is for analyzing a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")
			configLoging(verbose)
			walk()
			jsn, _ := json.MarshalIndent(errors, "", "  ")
			fname := app + "_analyze_errors.json"
			err := ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				log.Errorf("Unable to write to file %s (%v)", fname, err)
			}
		},
	}

	var verbose bool
	var rootCmd = &cobra.Command{Use: app}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug)")
	rootCmd.AddCommand(cmdAnalyze)
	err = rootCmd.Execute()
	if err != nil {
		log.Errorf("Unable to execute root command (%v)", err)
	}
	s := summarizeResults(errors)
	jsn, _ := json.MarshalIndent(s, "", "  ")
	fname := app + "_analyze_summary.json"
	err = ioutil.WriteFile(fname, jsn, 0600)
	if err != nil {
		log.Errorf("Unable to write to file %s (%v)", fname, err)
	}

	export := ErrorsExport{
		ComponentType: info.Type,
		ComponentName: info.Name,
		Errors:        make(map[string]ErrorExport),
	}
	for k, v := range errors.LiteralCodes {
		if len(v) > 1 {
			log.Warnf("duplicate code %s", k)
		}
		e := v[0]
		export.Errors[k] = ErrorExport{
			Name:                 e.Name,
			Code:                 e.Code,
			Severity:             "none",
			ShortDescription:     "might contain newlines (JSON encoded)",
			LongDescription:      "might contain newlines (JSON encoded)",
			ProbableCause:        "might contain newlines (JSON encoded)",
			SuggestedRemediation: "might contain newlines (JSON encoded)",
		}
	}
	jsn, _ = json.MarshalIndent(export, "", "  ")
	fname = app + "_errors_export.json"
	err = ioutil.WriteFile(fname, jsn, 0600)
	if err != nil {
		log.Errorf("Unable to write to file %s (%v)", fname, err)
	}
}
