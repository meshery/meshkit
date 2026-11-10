package registry

import (
	"io"
	"path/filepath"
	"reflect"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
)

func TestParseModelsSheet(t *testing.T) {
	previousLog := Log
	Log = SetupLogger("model-csv-test", false, io.Discard)
	t.Cleanup(func() { Log = previousLog })

	newHelper := func(t *testing.T, fixture string) *ModelCSVHelper {
		t.Helper()
		path := filepath.Join("testdata", fixture)
		helper, err := NewModelCSVHelper("", "Models", 0, path)
		if err != nil {
			t.Fatal(err)
		}
		return helper
	}

	t.Run("returns a read error after parsing a model", func(t *testing.T) {
		helper := newHelper(t, "models-read-error.csv")
		err := helper.ParseModelsSheet(false, "")
		if err == nil {
			t.Fatal("expected the CSV read error after the valid model row")
		}
		if code := meshkiterrors.GetCode(err); code != utils.ErrReadFileCode {
			t.Fatalf("expected read error code %q, got %q: %v", utils.ErrReadFileCode, code, err)
		}
		if len(helper.Models) != 1 || helper.Models[0].Model != "kubernetes" {
			t.Fatalf("expected the model before the read failure, got %#v", helper.Models)
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
			helper := newHelper(t, "models.csv")
			if err := helper.ParseModelsSheet(false, test.model); err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(helper.Models))
			for _, m := range helper.Models {
				got = append(got, m.Model)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parsed models = %#v, want %#v", got, test.want)
			}
		})
	}
}
