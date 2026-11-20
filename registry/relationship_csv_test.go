package registry

import (
	"io"
	"path/filepath"
	"reflect"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
)

func TestParseRelationshipsSheet(t *testing.T) {
	previousLog := Log
	Log = SetupLogger("relationship-csv-test", false, io.Discard)
	t.Cleanup(func() { Log = previousLog })

	newHelper := func(t *testing.T, fixture string) *RelationshipCSVHelper {
		t.Helper()
		path := filepath.Join("testdata", fixture)
		helper, err := NewRelationshipCSVHelper("", "Relationships", 0, path)
		if err != nil {
			t.Fatal(err)
		}
		return helper
	}

	t.Run("returns a read error after parsing a relationship", func(t *testing.T) {
		helper := newHelper(t, "relationships-read-error.csv")
		err := helper.ParseRelationshipsSheet("")
		if err == nil {
			t.Fatal("expected the CSV read error after the valid relationship row")
		}
		if code := meshkiterrors.GetCode(err); code != utils.ErrReadFileCode {
			t.Fatalf("expected read error code %q, got %q: %v", utils.ErrReadFileCode, code, err)
		}
		if len(helper.Relationships) != 1 || helper.Relationships[0].Model != "kubernetes" {
			t.Fatalf("expected the relationship before the read failure, got %#v", helper.Relationships)
		}
	})

	for _, test := range []struct {
		name  string
		model string
		want  []string
	}{
		{
			name: "retains every row on successful completion",
			want: []string{"kubernetes", "istio", "kubernetes"},
		},
		{
			name:  "filters by model without dropping the last matching row",
			model: "kubernetes",
			want:  []string{"kubernetes", "kubernetes"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			helper := newHelper(t, "relationships.csv")
			if err := helper.ParseRelationshipsSheet(test.model); err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(helper.Relationships))
			for _, r := range helper.Relationships {
				got = append(got, r.Model)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parsed relationships = %#v, want %#v", got, test.want)
			}
		})
	}
}
