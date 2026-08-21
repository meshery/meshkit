package coder

import (
	"bytes"
	"testing"

	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
)

func TestCheckLogic(t *testing.T) {
	tests := []struct {
		name       string
		baseline   mesherr.InfoAll
		current    mesherr.InfoAll
		wantErrors bool
	}{
		{
			name: "New placeholder passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "replace_me", CodeIsInt: false},
				},
			},
			wantErrors: false,
		},
		{
			name: "New unique manual code fails",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1001", CodeIsInt: true},
				},
			},
			wantErrors: true,
		},
		{
			name: "New manual code reusing existing code fails",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1000", CodeIsInt: true}, // Old name still exists, reuse fails
				},
			},
			wantErrors: true,
		},
		{
			name: "Unchanged existing code passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			wantErrors: false,
		},
		{
			name: "Rename existing error fails",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrRenamed", Code: "1000", CodeIsInt: true}, // Old name removed, but new name has manual code -> fails
				},
			},
			wantErrors: true,
		},
		{
			name: "Move existing error passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true, Path: "pkg/old/error.go"},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true, Path: "pkg/new/error.go"},
				},
			},
			wantErrors: false,
		},
		{
			name: "Multiple additions",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew1", Code: "replace_me", CodeIsInt: false},
					{Name: "ErrNew2", Code: "replace_me", CodeIsInt: false},
				},
			},
			wantErrors: false,
		},
		{
			name: "Mixed additions with one invalid",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew1", Code: "replace_me", CodeIsInt: false},
					{Name: "ErrNew2", Code: "1001", CodeIsInt: true},
				},
			},
			wantErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ValidateNewErrors(tt.baseline, tt.current, &buf)
			if (err != nil) != tt.wantErrors {
				t.Errorf("ValidateNewErrors() error = %v, wantErrors %v", err, tt.wantErrors)
			}
		})
	}
}
