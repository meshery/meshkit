package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ErrorInfo struct {
	Name          string `yaml:"name" json:"name"`
	Code          string `yaml:"code" json:"code"`
	CodeIsLiteral bool   `yaml:"codeIsLiteral" json:"codeIsLiteral"`
	CodeIsInt     bool   `yaml:"codeIsInt" json:"codeIsInt"`
	Path          string `yaml:"path" json:"path"`
}

type Errors struct {
	Entries       []ErrorInfo
	LiteralCodes  map[string][]ErrorInfo
	CallExprCodes []ErrorInfo
}

func NewErrors() *Errors {
	return &Errors{LiteralCodes: make(map[string][]ErrorInfo)}
}

var errors = NewErrors()

func init_logging(verbose bool) {
	//log.SetFormatter(&log.JSONFormatter{})

	log.SetOutput(os.Stdout)

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
	matched, _ := regexp.MatchString("Err[A-Z]", name)
	return matched
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func analyse(path string) error {
	logger := log.WithFields(log.Fields{"path": path})
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			logger.WithFields(log.Fields{"decl": "func", "name": fmt.Sprintf("%s", decl.Name)}).Debug("ast declaration")
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
				err := analyse(path)
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

	var cmdAnalyze = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze is for analyzing a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")
			init_logging(verbose)
			walk()
			for i, v := range errors.Entries {
				jsn, _ := json.Marshal(v)
				fmt.Printf("%d: %s\n", i, jsn)
			}
			for i, v := range errors.CallExprCodes {
				jsn, _ := json.Marshal(v)
				fmt.Printf("%d: %s\n", i, jsn)
			}
			jsn, _ := json.Marshal(errors)
			fmt.Println(string(jsn))
		},
	}

	var verbose bool
	var rootCmd = &cobra.Command{Use: "errorutil"}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output (debug)")
	rootCmd.AddCommand(cmdAnalyze)
	rootCmd.Execute()
}
