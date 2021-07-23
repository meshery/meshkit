package database

import "github.com/layer5io/meshkit/errors"

var (
	ErrNoneDatabaseCode           = "11010"
	ErrDatabaseOpenCode           = "11011"
	ErrSQLMapUnmarshalJSONCode    = "11012"
	ErrSQLMapUnmarshalTextCode    = "11013"
	ErrSQLMapMarshalValueCode     = "11014"
	ErrSQLMapUnmarshalScannedCode = "11015"
	ErrSQLMapInvalidScanCode      = "11016"

	ErrNoneDatabase      = errors.New(ErrNoneDatabaseCode, errors.Alert, []string{"No Database selected"}, []string{}, []string{"database name is empty"}, []string{"Input a name for the database"})
	ErrSQLMapInvalidScan = errors.New(ErrSQLMapUnmarshalScannedCode, errors.Alert, []string{"invalid data type: expected []byte"}, []string{}, []string{}, []string{})
)

func ErrDatabaseOpen(err error) error {
	return errors.New(ErrDatabaseOpenCode, errors.Alert, []string{"Unable to open database", err.Error()}, []string{err.Error()}, []string{"Database is unreachable"}, []string{"Make sure your database is reachable"})
}

// ErrSQLMapUnmarshalJSON represents the error which will occur when the native SQL driver
// will fail to unmarshal the JSON
func ErrSQLMapUnmarshalJSON(err error) error {
	return errors.New(ErrSQLMapUnmarshalJSONCode, errors.Alert, []string{"failed to unmarshal json", err.Error()}, []string{err.Error()}, []string{}, []string{})
}

// ErrSQLMapUnmarshalJSON represents the error which will occur when the native SQL driver
// will fail to unmarshal the text
func ErrSQLMapUnmarshalText(err error) error {
	return errors.New(ErrSQLMapUnmarshalTextCode, errors.Alert, []string{"failed to unmarshal text", err.Error()}, []string{err.Error()}, []string{}, []string{})
}

// ErrSQLMapMarshalValue represents the error which will occur when the native SQL driver
// will fail to marshal the value
func ErrSQLMapMarshalValue(err error) error {
	return errors.New(ErrSQLMapMarshalValueCode, errors.Alert, []string{"failed to marshal value", err.Error()}, []string{err.Error()}, []string{}, []string{})
}

// ErrSQLMapUnmarshalScanned represents the error which will occur when the native SQL driver
// will fail to unmarshal the scanned data
func ErrSQLMapUnmarshalScanned(err error) error {
	return errors.New(ErrSQLMapUnmarshalScannedCode, errors.Alert, []string{"failed to unmarshal scanned data", err.Error()}, []string{err.Error()}, []string{}, []string{})
}
