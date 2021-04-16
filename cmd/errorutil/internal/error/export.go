package error

import (
	"encoding/json"
	"io/ioutil"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	log "github.com/sirupsen/logrus"
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

func Export(componentInfo component.ComponentInfo, errorsInfo *ErrorsInfo) {
	export := ErrorsExport{
		ComponentType: componentInfo.Type,
		ComponentName: componentInfo.Name,
		Errors:        make(map[string]ErrorExport),
	}
	for k, v := range errorsInfo.LiteralCodes {
		if len(v) > 1 {
			log.Warnf("duplicate code %s", k)
		}
		e := v[0]
		if _, ok := strconv.Atoi(e.Code); ok == nil {
			export.Errors[k] = ErrorExport{
				Name:                 e.Name,
				Code:                 e.Code,
				Severity:             "",
				ShortDescription:     "",
				LongDescription:      "",
				ProbableCause:        "",
				SuggestedRemediation: "",
			}
		} else {
			log.Warnf("non-integer code %s", k)
		}
	}
	jsn, _ := json.MarshalIndent(export, "", "  ")
	fname := config.App + "_errors_export.json"
	err := ioutil.WriteFile(fname, jsn, 0600)
	if err != nil {
		log.Errorf("Unable to write to file %s (%v)", fname, err)
	}
}
