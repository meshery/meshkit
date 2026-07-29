package coder

import (
	"encoding/json"
	"os"
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
			bBytes, _ := json.Marshal(tt.baseline)
			cBytes, _ := json.Marshal(tt.current)

			bFile, _ := os.CreateTemp("", "baseline*.json")
			cFile, _ := os.CreateTemp("", "current*.json")
			defer os.Remove(bFile.Name())
			defer os.Remove(cFile.Name())

			bFile.Write(bBytes)
			cFile.Write(cBytes)
			bFile.Close()
			cFile.Close()

			cmd := commandCheck()
			cmd.SetArgs([]string{bFile.Name(), cFile.Name()})
			cmd.SetOut(os.Stdout)
			cmd.SetErr(os.Stderr)
			err := cmd.Execute()
			if (err != nil) != tt.wantErrors {
				t.Errorf("commandCheck() error = %v, wantErrors %v", err, tt.wantErrors)
			}
		})
	}
}
