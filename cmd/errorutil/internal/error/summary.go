package error

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
)

type analysisSummary struct {
	MinCode              int                 `yaml:"min_code" json:"min_code"`                              // the smallest error code (an int)
	MaxCode              int                 `yaml:"max_code" json:"max_code"`                              // the biggest error code (an int)
	Duplicates           map[string][]string `yaml:"duplicates" json:"duplicates"`                          // duplicate error codes
	CallExprCodes        []string            `yaml:"call_expr_codes" json:"call_expr_codes"`                // codes set by call expressions instead of literals
	IntCodes             []int               `yaml:"int_codes" json:"int_codes"`                            // all error codes as integers
	DeprecatedNewDefault []string            `yaml:"deprecated_new_default" json:"deprecated_new_default" ` // list of files with usage of deprecated NewDefault func
}

func SummarizeAnalysis(infoAll *InfoAll, outputDir string) error {
	maxInt := int(^uint(0) >> 1)
	summary := &analysisSummary{MinCode: maxInt, MaxCode: -maxInt - 1, Duplicates: make(map[string][]string)}
	for k, v := range infoAll.LiteralCodes {
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
				if !contains(summary.IntCodes, i) {
					summary.IntCodes = append(summary.IntCodes, i)
				}
			}
			if _, ok := summary.Duplicates[k]; ok {
				summary.Duplicates[k] = append(summary.Duplicates[k], e.Name)
			}
		}
	}
	for _, v := range infoAll.CallExprCodes {
		summary.CallExprCodes = append(summary.CallExprCodes, v.Name)
	}
	summary.DeprecatedNewDefault = infoAll.DeprecatedNewDefault
	jsn, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	fname := filepath.Join(outputDir, config.App+"_analyze_summary.json")
	return ioutil.WriteFile(fname, jsn, 0600)
}

func contains(s []int, str int) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
