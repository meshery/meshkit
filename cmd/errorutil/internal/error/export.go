package error

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"
	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	log "github.com/sirupsen/logrus"
)

// Error is used to export Error for e.g. documentation purposes.
//
// Type Error (errors/types.go) is not reused in order to avoid tight coupling between code and documentation of errors, e.g. on Meshery website.
// It is good practice not to use internal data types in integrations; one should in general transform between internal and external models.
// DDD calls this anti-corruption layer.
// One reason is that one might like to have a different representation externally, e.g. severity 'info' instead of '1'.
// Another one is that it is often desirable to be able to change the internal representation without the need for the consumer
// (in this case, the meshery doc) to have to adjust quickly in order to be able to handle updated content.
// The lifecycles of producers and consumers should not be tightly coupled.
type Error struct {
	Name                 string `yaml:"name" json:"name"`                                   // the name of the error code variable, e.g. "ErrInstallMesh", not guaranteed to be unique as it is package scoped
	Code                 string `yaml:"code" json:"code"`                                   // the code, an int, but exported as string, e.g. "1001", guaranteed to be unique per component-type:component-name
	Severity             string `yaml:"severity" json:"severity"`                           // a textual representation of the type Severity (errors/types.go), i.e. "none", "alert", etc
	LongDescription      string `yaml:"long_description" json:"long_description"`           // might contain newlines (JSON encoded)
	ShortDescription     string `yaml:"short_description" json:"short_description"`         // might contain newlines (JSON encoded)
	ProbableCause        string `yaml:"probable_cause" json:"probable_cause"`               // might contain newlines (JSON encoded)
	SuggestedRemediation string `yaml:"suggested_remediation" json:"suggested_remediation"` // might contain newlines (JSON encoded)
}

// externalAll is used to export all Errors including information about the component for e.g. documentation purposes.
type externalAll struct {
	ComponentName string           `yaml:"component_name" json:"component_name"` // component type, e.g. "adapter"
	ComponentType string           `yaml:"component_type" json:"component_type"` // component name, e.g. "kuma"
	Errors        map[string]Error `yaml:"errors" json:"errors"`                 // map of all errors with key = code
}

func Export(componentInfo *component.Info, infoAll *InfoAll, outputDir string) error {
	fname := filepath.Join(outputDir, config.App+"_errors_export.json")
	export := externalAll{
		ComponentType: componentInfo.Type,
		ComponentName: componentInfo.Name,
		Errors:        make(map[string]Error),
	}
	for k, v := range infoAll.LiteralCodes {
		if len(v) > 1 {
			log.Errorf("duplicate code '%s' - skipping export", k)
			continue
		}
		e := v[0]
		if _, err := strconv.Atoi(e.Code); err != nil {
			log.Errorf("non-integer code '%s' - skipping export", k)
			continue
		}
		// default value used if validations below fail
		export.Errors[k] = Error{
			Name:                 e.Name,
			Code:                 e.Code,
			Severity:             "",
			ShortDescription:     "",
			LongDescription:      "",
			ProbableCause:        "",
			SuggestedRemediation: "",
		}
		// were details for this error generated using errors.New(...)?
		if _, ok := infoAll.Errors[e.Name]; ok {
			log.Infof("error details found for error name '%s' and code '%s'", e.Name, e.Code)
			// no duplicates?
			if len(infoAll.Errors[e.Name]) == 1 {
				details := infoAll.Errors[e.Name][0]
				export.Errors[k] = Error{
					Name:                 details.Name,
					Code:                 e.Code,
					Severity:             details.Severity,
					ShortDescription:     details.ShortDescription,
					LongDescription:      details.LongDescription,
					ProbableCause:        details.ProbableCause,
					SuggestedRemediation: details.SuggestedRemediation,
				}
			} else {
				log.Errorf("duplicate error details for error name '%s' and code '%s'", e.Name, e.Code)
			}
		} else {
			log.Warnf("no error details found for error name '%s' and code '%s'", e.Name, e.Code)
		}
	}
	jsn, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}
	log.Infof("exporting to %s", fname)
	return ioutil.WriteFile(fname, jsn, 0600)
}
