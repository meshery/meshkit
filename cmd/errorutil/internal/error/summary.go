package error

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	meshlogger "github.com/layer5io/meshkit/cmd/errorutil/logger"

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

	for code, errors := range infoAll.LiteralCodes {
		if len(errors) > 1 {
			_, ok := summary.DuplicateCodes[code]
			if !ok {
				summary.DuplicateCodes[code] = []string{}
			}
		}
		for _, e := range errors {
			if e.CodeIsInt {
				i, _ := strconv.Atoi(e.Code)
				if i < summary.MinCode {
					summary.MinCode = i
				}
				if i > summary.MaxCode {
					summary.MaxCode = i
				}
				if !intSliceContains(summary.IntCodes, i) {
					summary.IntCodes = append(summary.IntCodes, i)
				}
			}
			if _, ok := summary.DuplicateCodes[code]; ok {
				summary.DuplicateCodes[code] = append(summary.DuplicateCodes[code], e.Name)
				meshlogger.Errorf(logger, "duplicate error code '%s', name: '%s'", code, e.Name)
			}
		}
	}

	if summary.NextCode <= summary.MaxCode {
		meshlogger.Errorf(logger, "component_info.next_error_code '%v' is lower than or equal to highest used code '%v'", summary.NextCode, summary.MaxCode)
	}
	sort.Ints(summary.IntCodes)
	for k, v := range infoAll.Errors {
		if len(v) > 1 {
			summary.DuplicateNames = append(summary.DuplicateNames, k)
			meshlogger.Errorf(logger, "duplicate error code name '%s'", k)
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
	meshlogger.Infof(logger, "writing summary to %s", fname)
	return os.WriteFile(fname, jsn, 0600)
}

func intSliceContains(s []int, i int) bool {
	for _, v := range s {
		if v == i {
			return true
		}
	}
	return false
}
