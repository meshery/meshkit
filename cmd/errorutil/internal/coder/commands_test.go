package coder

import (
	"bytes"
	"testing"

	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
)

func TestCheckLogic(t *testing.T) {
	tests := []struct {
		name         string
		baseline     mesherr.InfoAll
		current      mesherr.InfoAll
		baselineNext int
		currentNext  int
		wantErrors   bool
	}{
		// 1. new + replace_me -> PASS
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
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 2. single legitimate local allocation -> PASS
		{
			name: "Single legitimate local allocation passes",
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
			baselineNext: 1001,
			currentNext:  1002,
			wantErrors:   false,
		},
		// 3. multiple legitimate local allocations -> PASS
		{
			name: "Multiple legitimate local allocations pass",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew1", Code: "1001", CodeIsInt: true},
					{Name: "ErrNew2", Code: "1002", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1003,
			wantErrors:   false,
		},
		// 4. manually hardcoded integer -> FAIL
		{
			name: "Manually hardcoded integer without metadata fails",
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
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   true,
		},
		// 5. vanity/out-of-range integer -> FAIL
		{
			name: "Vanity or out-of-range integer fails",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "2000", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1002,
			wantErrors:   true, // Code 2000 is not in [1001, 1002)
		},
		// 6. counter regression -> FAIL
		{
			name: "Counter regression fails",
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
			baselineNext: 1001,
			currentNext:  999, // Regression!
			wantErrors:   true,
		},
		// 7. duplicate code in current state (NOT in baseline) -> FAIL
		{
			name: "Duplicate code introduced by PR fails",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1000", CodeIsInt: true}, // Duplicate
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   true,
		},
		// 7.5. duplicate code inherited from baseline -> PASS
		{
			name: "Existing duplicate inherited from baseline passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrA", Code: "1000", CodeIsInt: true},
					{Name: "ErrB", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrA", Code: "1000", CodeIsInt: true},
					{Name: "ErrB", Code: "1000", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 8. no change -> PASS
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
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 9. pure rename preserving existing code -> PASS
		{
			name: "Rename existing error preserving code passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrRenamed", Code: "1000", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 10. rename + one genuinely new allocation -> PASS
		{
			name: "Rename + genuinely new allocation passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrRenamed", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1001", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1002,
			wantErrors:   false,
		},
		// 10.5. delete existing error + reuse its code for a new error -> PASS
		{
			name: "Delete old error and reuse its code passes (indistinguishable from rename)",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOldCode", Code: "1463", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrNewCode", Code: "1463", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 11. allocate-then-delete -> PASS
		// 12. over-advanced counter -> PASS with explanatory comment
		{
			name: "Allocate-then-delete / Over-advanced counter passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1002", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1005,
			// Explanation: The counter has advanced from 1001 to 1005.
			// Code 1002 falls within the [1001, 1005) range.
			// This represents a legitimate workflow where an error code (e.g. 1001)
			// was allocated, but later deleted from the PR. We shouldn't enforce
			// strict equality between the count of new errors and currentNext - baselineNext.
			wantErrors:   false,
		},
		// 13. unrelated integer/string constant -> unaffected
		{
			name: "Unrelated non-int or placeholder code passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrString", Code: "some_string_val", CodeIsInt: false},
				},
			},
			baselineNext: 1001,
			currentNext:  1001,
			wantErrors:   false,
		},
		// 14. existing code moved between files/packages -> PASS
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
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   false,
		},
		// 15. Two new errors both left as replace_me (simulating the 'place_' normalization collision)
		{
			name: "Multiple new placeholders simulating normalization collision passes",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrExistingCode", Code: "1463", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrExistingCode", Code: "1463", CodeIsInt: true},
					{Name: "ErrNewOneCode", Code: "place_", CodeIsInt: false},
					{Name: "ErrNewTwoCode", Code: "place_", CodeIsInt: false},
				},
			},
			baselineNext: 1464,
			currentNext:  1464,
			wantErrors:   false,
		},
		// 16. Two new errors both hardcoded to the same existing integer
		{
			name: "Two new errors hardcoded to the same integer fail",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrExistingCode", Code: "1463", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrExistingCode", Code: "1463", CodeIsInt: true},
					{Name: "ErrNewOneCode", Code: "1463", CodeIsInt: true},
					{Name: "ErrNewTwoCode", Code: "1463", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ValidateNewErrors(tt.baseline, tt.current, tt.baselineNext, tt.currentNext, &buf)
			if (err != nil) != tt.wantErrors {
				t.Errorf("ValidateNewErrors() error = %v, wantErrors %v. Output: %s", err, tt.wantErrors, buf.String())
			}
		})
	}
}
