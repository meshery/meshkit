package registry

import (
	"io"
	"path/filepath"
	"reflect"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils"
)

func TestParseComponentsSheet(t *testing.T) {
	previousLog := Log
	Log = SetupLogger("component-csv-test", false, io.Discard)
	t.Cleanup(func() { Log = previousLog })

	newHelper := func(t *testing.T, fixture string) *ComponentCSVHelper {
		t.Helper()
		path := filepath.Join("testdata", fixture)
		helper, err := NewComponentCSVHelper("", "Components", 0, path)
		if err != nil {
			t.Fatal(err)
		}
		return helper
	}

	t.Run("returns a read error after parsing a component", func(t *testing.T) {
		helper := newHelper(t, "components-read-error.csv")
		err := helper.ParseComponentsSheet("")
		if err == nil {
			t.Fatal("expected the CSV read error after the valid component row")
		}
		if code := meshkiterrors.GetCode(err); code != utils.ErrReadFileCode {
			t.Fatalf("expected read error code %q, got %q: %v", utils.ErrReadFileCode, code, err)
		}
		components := helper.Components["meshery"]["kubernetes"]
		if len(components) != 1 || components[0].Component != "Pod" {
			t.Fatalf("expected the component before the read failure, got %#v", components)
		}
	})

	for _, test := range []struct {
		name  string
		model string
		want  map[string][]string
	}{
		{
			name: "retains every row on successful completion",
			want: map[string][]string{
				"meshery/kubernetes": {"Pod", "Deployment"},
				"meshery/istio":      {"VirtualService"},
			},
		},
		{
			name:  "filters by model without dropping the last matching row",
			model: "kubernetes",
			want:  map[string][]string{"meshery/kubernetes": {"Pod", "Deployment"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			helper := newHelper(t, "components.csv")
			if err := helper.ParseComponentsSheet(test.model); err != nil {
				t.Fatal(err)
			}
			got := make(map[string][]string)
			for registrant, models := range helper.Components {
				for model, components := range models {
					for _, component := range components {
						key := registrant + "/" + model
						got[key] = append(got[key], component.Component)
					}
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parsed components = %#v, want %#v", got, test.want)
			}
		})
	}
}
