package logger

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDatabase_Trace(t *testing.T) {
	opts := Options{
		Format:           TerminalLogFormat,
		LogLevel:         int(logrus.DebugLevel),
		EnableCallerInfo: false,
	}
	log, err := New("testapp", opts)
	assert.NoError(t, err)
	l := log.(*Logger)

	var outBuffer, errBuffer bytes.Buffer
	l.UpdateLogOutput(&outBuffer)
	l.UpdateErrorLogOutput(&errBuffer)

	db := l.DatabaseLogger()

	// A successful query is forwarded to the default handler.
	db.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT * FROM meshery_patterns", 3
	}, nil)
	assert.Contains(t, outBuffer.String(), "SELECT * FROM meshery_patterns")
	assert.Contains(t, outBuffer.String(), "[rows:3]")
	outBuffer.Reset()

	// A failed query is forwarded to the error handler, along with the error.
	db.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT * FROM missing_table", 0
	}, errors.New("no such table: missing_table"))
	assert.Contains(t, errBuffer.String(), "SELECT * FROM missing_table")
	assert.Contains(t, errBuffer.String(), "no such table: missing_table")
	errBuffer.Reset()

	// GORM reports rows as -1 when a row count does not apply to the statement.
	db.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "CREATE TABLE t (id int)", -1
	}, nil)
	assert.Contains(t, outBuffer.String(), "[rows:-]")
}
