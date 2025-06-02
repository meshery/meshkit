package validator

import (
	"encoding/json"
	"sync"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/schemas"
)

var (
	ErrValidateCode = ""
	_      = "components.schemas"
	cueschema       cue.Value
	mx              sync.Mutex
	isSchemaLoaded  bool
)

func loadSchema() error {
	mx.Lock()
	defer func() {
		mx.Unlock()
	}()

	if isSchemaLoaded {
		return nil
	}

	file, err := schemas.Schemas.Open("schemas/constructs/v1beta1/designs.json")
	if err != nil {
		return utils.ErrReadFile(err, "schemas/constructs/v1beta1/designs.json")
	}

	cueschema, err = utils.ConvertoCue(file)
	if err == nil {
		isSchemaLoaded = true
	}
	return err
}

func GetSchemaFor(resourceName string) (cue.Value, error) {
	var schema cue.Value
	schemaPathForResource := ""

	err := loadSchema()
	if err != nil {
		return schema, err
	}

	schema, err = utils.Lookup(cueschema, schemaPathForResource)
	if err != nil {
		return schema, err
	}

	byt, err := schema.MarshalJSON()
	if err != nil {
		return schema, utils.ErrMarshal(err)
	}

	schema, err = utils.JsonSchemaToCue(string(byt))

	log, _ := logger.New("test-validate", logger.Options{})
	log.Error(err)
	if err != nil {
		return schema, err
	}

	return schema, nil
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
