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

func summarizeResults(errors *Errors) *Summary {
	maxInt := int(^uint(0) >> 1)
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
}
