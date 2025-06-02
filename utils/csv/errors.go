package csv

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

var (
	ErrMarshalStructToCSVCode = "meshkit-11301"
	ErrReadCSVRowCode         = "meshkit-11306"
)

func ErrMarshalStructToCSV(err error) error {
	return errors.New(ErrMarshalStructToCSVCode, errors.Alert,
		[]string{"Failed to marshal struct to csv"},
		[]string{err.Error()},
		[]string{"The column names in your spreadsheet do not match the names in the struct.", " For example, the spreadsheet has a column named 'First Name' but the struct expects a column named 'firstname'. Please make sure the names match exactly."},
		[]string{"The column names in the spreadsheet do not match the names in the struct. Please make sure they are spelled exactly the same and use the same case (uppercase/lowercase).", "The value you are trying to convert is not of the expected type for the column. Please ensure it is a [number, string, date, etc.].", "The column names in your spreadsheet do not match the names in the struct. For example, the spreadsheet has a column named 'First Name' but the struct expects a column named 'firstname'. Please make sure the names match exactly."})
}

func ErrReadCSVRow(err error, obj string) error {
	return errors.New(ErrReadCSVRowCode, errors.Alert, []string{"error reading csv ", obj}, []string{err.Error()}, []string{fmt.Sprintf("the %s of the csv is broken", obj)}, []string{fmt.Sprintf("verify the csv %s", obj)})
}
