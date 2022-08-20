package utils

import (
	"fmt"
	"reflect"
	"testing"

	// "cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	// "cuelang.org/go/cue/errors"
)

var testSchema1 = `
name: string
task: string
properties?: {
  item: "array"
}
`
var testValue1 = `
name: 43
properties: item : "asf"
`

var testValue2 = `
name: string
google: int
prop: item: string
`

func TestValidate(t *testing.T) {
	var tests = []struct {
		schema string
		value  string
		want   bool
	}{
		{testSchema1, testValue1, false},
	}

	for _, tt := range tests {
		t.Run("validate", func(t *testing.T) {
			ctx := cuecontext.New()
			schema := ctx.CompileString(tt.schema)
			value := ctx.CompileString(tt.value)
			ans, errs := Validate(schema, value)
			fmt.Println("errs: ", errs)
			if ans != tt.want {
				t.Errorf("got %v, want %v", ans, tt.want)
			}
		})
	}
}

func TestGetNonConcreteFields(t *testing.T) {
	var tests = []struct {
		value string
		want  []string
	}{
		{testValue2, []string{"name", "google", "prop.item"}},
	}

	for _, tt := range tests {
		t.Run("validate", func(t *testing.T) {
			ctx := cuecontext.New()
			value := ctx.CompileString(tt.value)
			ans := GetNonConcreteFields(value)
			if !reflect.DeepEqual(ans, tt.want) {
				t.Errorf("got %v, want %v", ans, tt.want)
			}
		})
	}
}
