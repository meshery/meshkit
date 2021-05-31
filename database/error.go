package database

import "github.com/layer5io/meshkit/errors"

var (
	ErrNoneDatabaseCode           = "test"
	ErrDatabaseOpenCode           = "test"
	ErrSQLMapUnmarshalJSONCode    = "test"
	ErrSQLMapUnmarshalTextCode    = "test"
	ErrSQLMapMarshalValueCode     = "test"
	ErrSQLMapUnmarshalScannedCode = "test"
	ErrSQLMapInvalidScanCode      = "test"

	ErrNoneDatabase      = errors.NewDefault(ErrNoneDatabaseCode, "No Database selected")
	ErrSQLMapInvalidScan = errors.NewDefault(ErrSQLMapUnmarshalScannedCode, "invalid data type: expected []byte")
)

func ErrDatabaseOpen(err error) error {
	return errors.NewDefault(ErrDatabaseOpenCode, "Unable to open database", err.Error())
}

// ErrSQLMapUnmarshalJSON represents the error which will occur when the native SQL driver
// will fail to unmarshal the JSON
func ErrSQLMapUnmarshalJSON(err error) error {
	return errors.NewDefault(ErrSQLMapUnmarshalJSONCode, "failed to unmarshal json", err.Error())
}

// ErrSQLMapUnmarshalJSON represents the error which will occur when the native SQL driver
// will fail to unmarshal the text
func ErrSQLMapUnmarshalText(err error) error {
	return errors.NewDefault(ErrSQLMapUnmarshalTextCode, "failed to unmarshal text", err.Error())
}

// ErrSQLMapMarshalValue represents the error which will occur when the native SQL driver
// will fail to marshal the value
func ErrSQLMapMarshalValue(err error) error {
	return errors.NewDefault(ErrSQLMapMarshalValueCode, "failed to marshal value", err.Error())
}

// ErrSQLMapUnmarshalScanned represents the error which will occur when the native SQL driver
// will fail to unmarshal the scanned data
func ErrSQLMapUnmarshalScanned(err error) error {
	return errors.NewDefault(ErrSQLMapUnmarshalScannedCode, "failed to unmarshal scanned data", err.Error())
}
