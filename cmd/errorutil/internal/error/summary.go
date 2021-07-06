package error

import (
	"encoding/json"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
)

type analysisSummary struct {
	MinCode              int                 `yaml:"min_code" json:"min_code"`                              // the smallest error code (an int)
	MaxCode              int                 `yaml:"max_code" json:"max_code"`                              // the biggest error code (an int)
	NextCode             int                 `yaml:"next_code" json:"next_code"`                            // the next error code to use, taken from ComponentInfo
	DuplicateCodes       map[string][]string `yaml:"duplicate_codes" json:"duplicate_codes"`                // duplicate error codes
	DuplicateNames       []string            `yaml:"duplicate_names" json:"duplicate_names"`                // duplicate error names
	CallExprCodes        []string            `yaml:"call_expr_codes" json:"call_expr_codes"`                // codes set by call expressions instead of literals
	IntCodes             []int               `yaml:"int_codes" json:"int_codes"`                            // all error codes as integers
	DeprecatedNewDefault []string            `yaml:"deprecated_new_default" json:"deprecated_new_default" ` // list of files with usage of deprecated NewDefault func
}

// SummarizeAnalysis summarizes the analysis and writes it to the specified output directory.
func SummarizeAnalysis(componentInfo *component.Info, infoAll *InfoAll, outputDir string) error {
	maxInt := int(^uint(0) >> 1)
	summary := &analysisSummary{
		MinCode:              maxInt,
		MaxCode:              -maxInt - 1,
		NextCode:             componentInfo.NextErrorCode,
		DuplicateCodes:       make(map[string][]string),
		DuplicateNames:       []string{},
		CallExprCodes:        []string{},
		IntCodes:             []int{},
		DeprecatedNewDefault: []string{}}
	for k, v := range infoAll.LiteralCodes {
		if len(v) > 1 {
			_, ok := summary.DuplicateCodes[k]
			if !ok {
				summary.DuplicateCodes[k] = []string{}
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
			if _, ok := summary.DuplicateCodes[k]; ok {
				summary.DuplicateCodes[k] = append(summary.DuplicateCodes[k], e.Name)
				log.Errorf("duplicate error code '%s', name: '%s'", k, e.Name)
			}
		}
	}
	sort.Ints(summary.IntCodes)
	for k, v := range infoAll.Errors {
		if len(v) > 1 {
			summary.DuplicateNames = append(summary.DuplicateNames, k)
			log.Errorf("duplicate error code name '%s'", k)
		}
	}
	sort.Strings(summary.DuplicateNames)
	for _, v := range infoAll.CallExprCodes {
		summary.CallExprCodes = append(summary.CallExprCodes, v.Name)
	}
	sort.Strings(summary.CallExprCodes)
	summary.DeprecatedNewDefault = infoAll.DeprecatedNewDefault
	sort.Strings(summary.DeprecatedNewDefault)
	jsn, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	fname := filepath.Join(outputDir, config.App+"_analyze_summary.json")
	log.Infof("writing summary to %s", fname)
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
