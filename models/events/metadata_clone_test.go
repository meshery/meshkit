package events

import (
	"reflect"
	"testing"
)

type cloneInner struct {
	Tags   []string
	Labels map[string]string
}

type cloneOuter struct {
	Name  string
	Inner cloneInner
	Ptr   *cloneInner
	Any   interface{}
	Arr   [2][]int
}

type cloneNode struct {
	Name string
	Next *cloneNode
}

// Every reference-typed metadata value is deep-copied by Build: an in-place
// write to the value the builder holds after Build never reaches the built
// event.
func TestBuildDeepCopiesMetadataValues(t *testing.T) {
	cases := []struct {
		name   string
		value  func() interface{}
		mutate func(v interface{})
	}{
		{
			name:   "string slice",
			value:  func() interface{} { return []string{"a", "b"} },
			mutate: func(v interface{}) { v.([]string)[0] = "changed" },
		},
		{
			name:   "string map",
			value:  func() interface{} { return map[string]string{"k": "v"} },
			mutate: func(v interface{}) { v.(map[string]string)["k"] = "changed"; v.(map[string]string)["new"] = "x" },
		},
		{
			name:   "nested map and slice",
			value:  func() interface{} { return map[string]interface{}{"list": []interface{}{map[string]int{"n": 1}}} },
			mutate: func(v interface{}) { v.(map[string]interface{})["list"].([]interface{})[0].(map[string]int)["n"] = 2 },
		},
		{
			name: "nested struct",
			value: func() interface{} {
				return cloneOuter{Name: "o", Inner: cloneInner{Tags: []string{"t"}, Labels: map[string]string{"l": "1"}},
					Any: []string{"any"}, Arr: [2][]int{{1}, {2}}}
			},
			mutate: func(v interface{}) {
				o := v.(cloneOuter)
				o.Inner.Tags[0] = "changed"
				o.Inner.Labels["l"] = "changed"
				o.Any.([]string)[0] = "changed"
				o.Arr[1][0] = 99
			},
		},
		{
			name:  "pointer to struct",
			value: func() interface{} { return &cloneOuter{Name: "o", Ptr: &cloneInner{Tags: []string{"t"}}} },
			mutate: func(v interface{}) {
				p := v.(*cloneOuter)
				p.Name = "changed"
				p.Ptr.Tags[0] = "changed"
				p.Ptr.Labels = map[string]string{}
			},
		},
		{
			name:   "pointer to scalar",
			value:  func() interface{} { n := 7; return &n },
			mutate: func(v interface{}) { *v.(*int) = 8 },
		},
		{
			name: "event error",
			value: func() interface{} {
				cause, fix := []string{"cause"}, []string{"fix"}
				return EventError{Code: "c", ProbableCause: &cause, SuggestedRemediation: &fix}
			},
			mutate: func(v interface{}) {
				e := v.(EventError)
				(*e.ProbableCause)[0] = "changed"
				*e.SuggestedRemediation = append(*e.SuggestedRemediation, "more")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := tc.value()
			b := NewEvent().WithMetadata(map[string]interface{}{"v": held})
			ev := b.Build()
			tc.mutate(held)
			if want := tc.value(); !reflect.DeepEqual(ev.Metadata["v"], want) {
				t.Errorf("built event metadata = %#v, want %#v", ev.Metadata["v"], want)
			}
			if again := b.Build(); !reflect.DeepEqual(again.Metadata["v"], held) {
				t.Errorf("builder lost its own metadata: %#v, want %#v", again.Metadata["v"], held)
			}
		})
	}
}

// Aliased and cyclic values keep their shape in the copy and do not recurse
// forever; nil values stay nil.
func TestCloneMetadataAliasesCyclesAndNils(t *testing.T) {
	a := &cloneNode{Name: "a"}
	a.Next = &cloneNode{Name: "b", Next: a}
	shared := []string{"s"}
	var nilMap map[string]string
	var nilPtr *cloneInner
	got := cloneMetadata(map[string]interface{}{
		"cycle": a, "x": shared, "y": shared, "nilMap": nilMap, "nilPtr": nilPtr, "nil": nil,
	})

	ca := got["cycle"].(*cloneNode)
	if ca == a || ca.Next == a.Next || ca.Next.Next != ca {
		t.Errorf("cycle not cloned with its shape: %p -> %p -> %p", ca, ca.Next, ca.Next.Next)
	}
	x, y := got["x"].([]string), got["y"].([]string)
	x[0] = "changed"
	if y[0] != "changed" || shared[0] != "s" {
		t.Errorf("aliased slices: y=%v shared=%v, want y aliased to x and original untouched", y, shared)
	}
	if got["nilMap"].(map[string]string) != nil || got["nilPtr"].(*cloneInner) != nil || got["nil"] != nil {
		t.Errorf("nil values not preserved: %#v", got)
	}
	if cloneMetadata(nil) != nil {
		t.Error("cloneMetadata(nil) != nil")
	}
}
