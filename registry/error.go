package registry

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

var (
	ErrMarshalStructToCSVCode     = "meshkit-11301"
	ErrAppendToSheetCode          = "meshkit-11302"
	ErrUpdateToSheetCode          = "meshkit-11303"
	ErrFileReadCode               = "meshkit-11304"
	ErrGeneratesModelCode         = "meshkit-11305"
	ErrReadCSVRowCode             = "meshkit-11306"
	ErrCSVFileNotFoundCode        = "meshkit-11307"
	ErrUpdateComponentsCode       = "meshkit-11308"
	ErrGeneratesComponentCode     = "meshkit-11309"
	ErrUpdateRelationshipFileCode = "meshkit-11310"
)

func ErrMarshalStructToCSV(err error) error {
	return errors.New(ErrMarshalStructToCSVCode, errors.Alert,
		[]string{"Failed to marshal struct to csv"},
		[]string{err.Error()},
		[]string{"The column names in your spreadsheet do not match the names in the struct.", " For example, the spreadsheet has a column named 'First Name' but the struct expects a column named 'firstname'. Please make sure the names match exactly."},
		[]string{"The column names in the spreadsheet do not match the names in the struct. Please make sure they are spelled exactly the same and use the same case (uppercase/lowercase).", "The value you are trying to convert is not of the expected type for the column. Please ensure it is a [number, string, date, etc.].", "The column names in your spreadsheet do not match the names in the struct. For example, the spreadsheet has a column named 'First Name' but the struct expects a column named 'firstname'. Please make sure the names match exactly."})
}

func ErrAppendToSheet(err error, id string) error {
	return errors.New(ErrAppendToSheetCode, errors.Alert,
		[]string{fmt.Sprintf("Failed to append data into sheet %s", id)},
		[]string{err.Error()},
		[]string{"Error occurred while appending to the spreadsheet", "The credential might be incorrect/expired"},
		[]string{"Ensure correct append range (A1 notation) is used", "Ensure correct credential is used"})
}

func ErrUpdateToSheet(err error, id string) error {
	return errors.New(ErrUpdateToSheetCode, errors.Alert,
		[]string{fmt.Sprintf("Failed to update data into sheet %s", id)},
		[]string{err.Error()},
		[]string{"Error occurred while updating to the spreadsheet", "The credential might be incorrect/expired"},
		[]string{"Ensure correct update range (A1 notation) is used", "Ensure correct credential is used"})
}

func ErrFileRead(err error) error {
	return errors.New(ErrFileReadCode, errors.Alert,
		[]string{"File read error"},
		[]string{err.Error()},
		[]string{"The provided file is not present or has an invalid path."},
		[]string{"To proceed, provide a valid file path with a valid file."})
}

func ErrGenerateModel(err error, modelName string) error {
	return errors.New(ErrGeneratesModelCode, errors.Alert, []string{fmt.Sprintf("error generating model: %s", modelName)}, []string{fmt.Sprintf("Error generating model: %s\n %s", modelName, err.Error())}, []string{"Registrant used for the model is not supported", "Verify the model's source URL.", "Failed to create a local directory in the filesystem for this model."}, []string{"Ensure that each kind of registrant used is a supported kind.", "Ensure correct model source URL is provided and properly formatted.", "Ensure sufficient permissions to allow creation of model directory."})
}

func ErrReadCSVRow(err error, obj string) error {
	return errors.New(ErrReadCSVRowCode, errors.Alert, []string{"error reading csv ", obj}, []string{err.Error()}, []string{fmt.Sprintf("the %s of the csv is broken", obj)}, []string{fmt.Sprintf("verify the csv %s", obj)})
}

func ErrCSVFileNotFound(path string) error {
	return errors.New(ErrCSVFileNotFoundCode, errors.Alert, []string{"error reading csv file", path}, []string{fmt.Sprintf("inside the directory %s either the model csv or component csv is missing or they are not of write format", path)}, []string{"Either or both model csv or component csv are absent, the csv is not of correct template"}, []string{fmt.Sprintf("verify both the csv are present in the directory:%s", path), "verify the csv template"})
}

func ErrUpdateComponent(err error, modelName, compName string) error {
	return errors.New(ErrUpdateComponentsCode, errors.Alert, []string{fmt.Sprintf("error updating component %s of model %s ", compName, modelName)}, []string{err.Error()}, []string{"Component does not exist", "Component definition is corrupted"}, []string{"Ensure existence of component, check for typo in component name", "Regenerate corrupted component"})
}

func ErrGenerateComponent(err error, modelName, compName string) error {
	return errors.New(ErrGeneratesComponentCode, errors.Alert, []string{"error generating comp %s of model %s", compName, modelName}, []string{err.Error()}, []string{}, []string{})
}

func ErrUpdateRelationshipFile(err error) error {
	return errors.New(ErrUpdateRelationshipFileCode, errors.Alert, []string{"error while comparing files"},
		[]string{err.Error()},
		[]string{"Error occurred while comapring the new file and the existing relationship file generated from the spreadsheet"},
		[]string{"Ensure that the new file is in the correct format and has the correct data"})
}
