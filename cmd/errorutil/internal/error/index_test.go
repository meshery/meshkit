package error

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"
	"github.com/meshery/schemas/models/v1beta3/errorcode"
)

// The index takes each definition's code from the exported entry's Code, never
// from the key the entry is filed under, which can still hold the code from
// before update allocated it.
func TestExportIndexUsesEntryCode(t *testing.T) {
	info := &component.Info{Name: "meshery-server", Type: "component"}
	exported := map[string]Error{
		"replace_m": {Name: "ErrFooCode", Code: "meshery-server-1034", ShortDescription: "Foo"},
		"1040":      {Name: "ErrBarCode", Code: "meshery-server-1033", ShortDescription: "Bar"},
	}
	all := NewInfoAll()
	all.Errors["ErrFooCode"] = []Error{{Name: "ErrFooCode", Severity: "Alert", Lines: Lines{ShortDescription: []string{"Foo"}}}}
	all.Errors["ErrBarCode"] = []Error{{Name: "ErrBarCode", Severity: "Alert", Lines: Lines{ShortDescription: []string{"Bar"}}}}
	dir := t.TempDir()
	if err := ExportIndex(info, exported, all, dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "errorutil"+IndexFileSuffix))
	if err != nil {
		t.Fatal(err)
	}
	var index errorcode.ErrorReferenceIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	var got [][2]string
	for _, d := range index.Errors {
		got = append(got, [2]string{d.Code, d.NumericCode})
	}
	want := [][2]string{{"meshery-server-1033", "1033"}, {"meshery-server-1034", "1034"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("index codes = %v, want %v", got, want)
	}
}
