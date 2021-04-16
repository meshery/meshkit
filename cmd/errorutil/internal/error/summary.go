package error

import (
	"encoding/json"
	"io/ioutil"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	log "github.com/sirupsen/logrus"
)

type Summary struct {
	MinCode    int                 `yaml:"minCode" json:"minCode"`
	MaxCode    int                 `yaml:"maxCode" json:"maxCode"`
	Duplicates map[string][]string `yaml:"duplicates" json:"duplicates"`
	IntCodes   []int               `yaml:"intCodes" json:"intCodes"`
}

func Summarize(errors *ErrorsInfo) {
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
	jsn, _ := json.MarshalIndent(summary, "", "  ")
	fname := config.App + "_analyze_summary.json"
	err := ioutil.WriteFile(fname, jsn, 0600)
	if err != nil {
		log.Errorf("Unable to write to file %s (%v)", fname, err)
	}
}
