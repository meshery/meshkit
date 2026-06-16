package csv

import (
	"os"
	"path/filepath"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
)

type mappedRow struct {
	Name string `json:"name"`
	Age  string `json:"person_age"`
}

type numericAgeRow struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestNewCSVParserReturnsWrappedReadErrorForMissingFiles(t *testing.T) {
	_, err := NewCSVParser[mappedRow](filepath.Join(t.TempDir(), "missing.csv"), 0, nil, nil)
	if err == nil {
		t.Fatal("expected NewCSVParser to return an error for a missing file")
	}
	if got := meshkiterrors.GetCode(err); got != utils.ErrReadFileCode {
		t.Fatalf("expected error code %q, got %q", utils.ErrReadFileCode, got)
	}
}

func TestExtractCols(t *testing.T) {
	t.Run("reads the configured header row", func(t *testing.T) {
		path := writeCSVFixture(t, "Name,Age,Include\nAlice,30,yes\n")
		parser, err := NewCSVParser[mappedRow](path, 0, nil, nil)
		if err != nil {
			t.Fatalf("NewCSVParser() returned error: %v", err)
		}

		cols, err := parser.ExtractCols(0)
		if err != nil {
			t.Fatalf("ExtractCols() returned error: %v", err)
		}

		if len(cols) != 3 {
			t.Fatalf("expected 3 columns, got %d", len(cols))
		}
		if cols[0] != "Name" || cols[1] != "Age" || cols[2] != "Include" {
			t.Fatalf("unexpected columns: %#v", cols)
		}
	})

	t.Run("wraps read errors when the header row is missing", func(t *testing.T) {
		path := writeCSVFixture(t, "Name,Age\n")
		parser, err := NewCSVParser[mappedRow](path, 0, nil, nil)
		if err != nil {
			t.Fatalf("NewCSVParser() returned error: %v", err)
		}

		_, err = parser.ExtractCols(1)
		if err == nil {
			t.Fatal("expected ExtractCols to return an error when the requested header row is missing")
		}
		if got := meshkiterrors.GetCode(err); got != utils.ErrReadFileCode {
			t.Fatalf("expected error code %q, got %q", utils.ErrReadFileCode, got)
		}
	})
}

func TestParseAppliesPredicateAndColumnMapping(t *testing.T) {
	path := writeCSVFixture(t, "Name,Age,Include\nAlice,30,yes\nBob,40,no\nCharlie,50,yes\n")
	parser, err := NewCSVParser[mappedRow](
		path,
		0,
		map[string]string{"Age": "person_age"},
		func(columns []string, currentRow []string) bool {
			return currentRow[2] == "yes"
		},
	)
	if err != nil {
		t.Fatalf("NewCSVParser() returned error: %v", err)
	}

	rows := make(chan mappedRow, 3)
	errs := make(chan error, 1)
	if err := parser.Parse(rows, errs); err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	select {
	case err := <-errs:
		t.Fatalf("did not expect parse errors, got %v", err)
	default:
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 parsed rows, got %d", len(rows))
	}

	first := <-rows
	second := <-rows
	if first.Name != "Alice" || first.Age != "30" {
		t.Fatalf("unexpected first row: %#v", first)
	}
	if second.Name != "Charlie" || second.Age != "50" {
		t.Fatalf("unexpected second row: %#v", second)
	}

	select {
	case <-parser.Context.Done():
	default:
		t.Fatal("expected parser context to be canceled after Parse returns")
	}
}

func TestParseSendsMarshalErrorsToErrorChannel(t *testing.T) {
	path := writeCSVFixture(t, "Name,Age\nAlice,not-a-number\n")
	parser, err := NewCSVParser[numericAgeRow](
		path,
		0,
		nil,
		func(columns []string, currentRow []string) bool {
			return true
		},
	)
	if err != nil {
		t.Fatalf("NewCSVParser() returned error: %v", err)
	}

	rows := make(chan numericAgeRow, 1)
	errs := make(chan error, 1)
	if err := parser.Parse(rows, errs); err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected no successfully parsed rows, got %d", len(rows))
	}

	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("expected a non-nil marshal error")
		}
	default:
		t.Fatal("expected Parse to send a marshal error to the error channel")
	}
}

func TestParseWrapsCSVReadErrors(t *testing.T) {
	path := writeCSVFixture(t, "Name,Age\n\"Alice,30\n")
	parser, err := NewCSVParser[mappedRow](
		path,
		0,
		nil,
		func(columns []string, currentRow []string) bool {
			return true
		},
	)
	if err != nil {
		t.Fatalf("NewCSVParser() returned error: %v", err)
	}

	err = parser.Parse(make(chan mappedRow, 1), make(chan error, 1))
	if err == nil {
		t.Fatal("expected Parse to return a read error for malformed CSV data")
	}
	if got := meshkiterrors.GetCode(err); got != utils.ErrReadFileCode {
		t.Fatalf("expected error code %q, got %q", utils.ErrReadFileCode, got)
	}
}

func writeCSVFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write CSV fixture: %v", err)
	}

	return path
}
