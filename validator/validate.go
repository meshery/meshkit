package validator

import (
	"encoding/json"
	"fmt"
	"io"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas"
)

var (
	ErrValidateCode = ""
)

func GetSchemaFor(resourceName string) (cue.Value, error) {
	var schema cue.Value
	var sch string
	switch resourceName {
	case "design":
		sch = "constructs/v1beta1/design/design.json"
	case "catalog_data":
		sch = "constructs/v1alpha1/catalog_data.json"
	case "models":
		sch = "constructs/v1beta1/model/model.json"
	default:
		return schema, fmt.Errorf("no schema defined for resource: %s", resourceName)
	}
	file, err := schemas.Schemas.Open(fmt.Sprintf("schemas/%s", sch))
	if err != nil {
		return schema, err
	}
	byt, err := io.ReadAll(file)
	if err != nil {
		return schema, err
	}
	val, err := utils.JsonToCue(byt)
	if err != nil {
		return schema, err
	}
	return val, nil
}

func Validate(schema cue.Value, resourceValue interface{}) error {

	byt, err := json.Marshal(resourceValue)
	if err != nil {
		return utils.ErrMarshal(err)
	}

	cv, err := utils.JsonToCue(byt)
	if err != nil {
		return err
	}

	valid, errs := utils.Validate(schema, cv)
	if !valid {
		errs := convertCueErrorsToStrings(errs)
		return errors.New(ErrValidateCode,
			errors.Alert,
			[]string{"validation for the resource failed"},
			errs,
			[]string{}, []string{},
		)
	}
	return nil
}

func convertCueErrorsToStrings(errs []cueerrors.Error) []string {
	var res []string
	for _, err := range errs {

		_ = cueerrors.Sanitize(err)
	}
	for _, err2 := range errs {

		res = append(res, err2.Error())
	}
	return res
}