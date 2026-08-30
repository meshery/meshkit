package coder

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"
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
		wantOut      []string
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
					{Name: "ErrNew", Code: "replace_", CodeIsInt: false},
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
			wantErrors: false,
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
		// Third user of a pre-existing duplicate
		{
			name: "Third user of a pre-existing duplicate",
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
					{Name: "ErrC", Code: "1000", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   true,
			wantOut:      []string{"1000"},
		},
		// Stale/regressed baseline with NO new codes
		{
			name: "Stale/regressed baseline with NO new codes",
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
			baselineNext: 1050,
			currentNext:  1010,
			wantErrors:   true,
			wantOut:      []string{"1050", "1010"},
		},
		// Code equal to currentNext
		{
			name: "Code equal to currentNext",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew", Code: "1005", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1005,
			wantErrors:   true,
			wantOut:      []string{"1005"},
		},
		// Code one below baselineNext
		{
			name: "Code one below baselineNext",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "900", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "900", CodeIsInt: true},
					{Name: "ErrNew", Code: "1000", CodeIsInt: true},
				},
			},
			baselineNext: 1001,
			currentNext:  1010,
			wantErrors:   true,
			wantOut:      []string{"1000"},
		},
		// Only one of baselineNext/currentNext supplied
		{
			name: "Only one of baselineNext/currentNext supplied",
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
			currentNext:  1002,
			wantErrors:   true,
			wantOut:      []string{"1001"},
		},
		// Restore the mixed-additions case dropped from #1080
		{
			name: "Restore the mixed-additions case",
			baseline: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
				},
			},
			current: mesherr.InfoAll{
				Entries: []mesherr.Info{
					{Name: "ErrOld", Code: "1000", CodeIsInt: true},
					{Name: "ErrNew1", Code: "replace_", CodeIsInt: false},
					{Name: "ErrNew2", Code: "1002", CodeIsInt: true},
				},
			},
			baselineNext: -1,
			currentNext:  -1,
			wantErrors:   true,
			wantOut:      []string{"ErrNew2", "1002"},
		},

		// 15. Two new errors both left as replace_me (simulating two new replace_-normalized placeholders colliding)
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
					{Name: "ErrNewOneCode", Code: "replace_", CodeIsInt: false},
					{Name: "ErrNewTwoCode", Code: "replace_", CodeIsInt: false},
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
			for _, want := range tt.wantOut {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("ValidateNewErrors() output missing expected substring %q. Full output: %s", want, buf.String())
				}
			}
		})
	}
}

func TestCommandCheck(t *testing.T) {
	// Helper to write JSON files
	writeJSON := func(t *testing.T, dir, filename string, v interface{}) string {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// Helpers for standard payloads
	makeErrors := func(code string, codeIsInt bool) mesherr.InfoAll {
		return mesherr.InfoAll{
			Entries: []mesherr.Info{
				{Name: "ErrTest", Code: code, CodeIsInt: codeIsInt},
			},
		}
	}

	type analysisSummary struct {
		NextCode int `json:"next_code"`
	}

	tests := []struct {
		name           string
		setup          func(t *testing.T, dir string) (args []string)
		wantError      bool
		wantErrorMatch string
	}{
		// 1. Both summary flags with valid local allocation
		{
			name: "Both summary flags with valid local allocation",
			setup: func(t *testing.T, dir string) []string {
				err1 := makeErrors("1000", true)
				err2 := makeErrors("1001", true)
				bErr := writeJSON(t, dir, "b_err.json", err1)
				cErr := writeJSON(t, dir, "c_err.json", err2)

				comp1 := &component.Info{NextErrorCode: 1001}
				err := mesherr.SummarizeAnalysis(comp1, &err1, dir)
				if err != nil {
					t.Fatal(err)
				}
				os.Rename(filepath.Join(dir, "errorutil_analyze_summary.json"), filepath.Join(dir, "b_sum.json"))
				bSum := filepath.Join(dir, "b_sum.json")

				comp2 := &component.Info{NextErrorCode: 1002}
				err = mesherr.SummarizeAnalysis(comp2, &err2, dir)
				if err != nil {
					t.Fatal(err)
				}
				os.Rename(filepath.Join(dir, "errorutil_analyze_summary.json"), filepath.Join(dir, "c_sum.json"))
				cSum := filepath.Join(dir, "c_sum.json")

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError: false,
		},
		// 2. Both summary flags with invalid/manual integer
		{
			name: "Both summary flags with invalid manual integer",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("2000", true)) // Outside 1001..1002
				bSum := writeJSON(t, dir, "b_sum.json", analysisSummary{NextCode: 1001})
				cSum := writeJSON(t, dir, "c_sum.json", analysisSummary{NextCode: 1002})

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "error code validation failed; see details above",
		},
		// 3. No summary flags (legacy strict behavior)
		{
			name: "No summary flags",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true)) // Valid if local, but fails in strict

				return []string{bErr, cErr}
			},
			wantError:      true,
			wantErrorMatch: "error code validation failed; see details above",
		},
		// 4. Only baseline summary supplied
		{
			name: "Only baseline summary supplied",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				bSum := writeJSON(t, dir, "b_sum.json", analysisSummary{NextCode: 1001})

				return []string{
					"--baseline-summary", bSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "both --baseline-summary and --current-summary must be provided",
		},
		// 5. Only current summary supplied
		{
			name: "Only current summary supplied",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				cSum := writeJSON(t, dir, "c_sum.json", analysisSummary{NextCode: 1002})

				return []string{
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "both --baseline-summary and --current-summary must be provided",
		},
		// 6. Wrong file supplied as summary
		{
			name: "Wrong file supplied as summary",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))

				return []string{
					"--baseline-summary", bErr, // Passing error JSON instead of summary
					"--current-summary", cErr,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "next_code",
		},
		// 7. Missing baseline summary file
		{
			name: "Missing baseline summary file",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				cSum := writeJSON(t, dir, "c_sum.json", analysisSummary{NextCode: 1002})

				return []string{
					"--baseline-summary", filepath.Join(dir, "nonexistent.json"),
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "failed to read baseline summary",
		},
		// 8. Missing current summary file
		{
			name: "Missing current summary file",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				bSum := writeJSON(t, dir, "b_sum.json", analysisSummary{NextCode: 1001})

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", filepath.Join(dir, "nonexistent.json"),
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "failed to read current summary",
		},
		// 9. Malformed baseline summary JSON
		{
			name: "Malformed baseline summary JSON",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				cSum := writeJSON(t, dir, "c_sum.json", analysisSummary{NextCode: 1002})

				bSum := filepath.Join(dir, "b_sum.json")
				if err := os.WriteFile(bSum, []byte("not valid json"), 0644); err != nil {
					t.Fatal(err)
				}

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "failed to parse baseline summary",
		},
		// 10. Malformed current summary JSON
		{
			name: "Malformed current summary JSON",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				bSum := writeJSON(t, dir, "b_sum.json", analysisSummary{NextCode: 1001})

				cSum := filepath.Join(dir, "c_sum.json")
				if err := os.WriteFile(cSum, []byte("not valid json"), 0644); err != nil {
					t.Fatal(err)
				}

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "failed to parse current summary",
		},
		// 11. Summary with missing/zero next_code
		{
			name: "Summary with missing/zero next_code",
			setup: func(t *testing.T, dir string) []string {
				bErr := writeJSON(t, dir, "b_err.json", makeErrors("1000", true))
				cErr := writeJSON(t, dir, "c_err.json", makeErrors("1001", true))
				bSum := writeJSON(t, dir, "b_sum.json", analysisSummary{NextCode: 0})
				cSum := writeJSON(t, dir, "c_sum.json", analysisSummary{NextCode: 1002})

				return []string{
					"--baseline-summary", bSum,
					"--current-summary", cSum,
					bErr, cErr,
				}
			},
			wantError:      true,
			wantErrorMatch: "next_code",
		},
		// 12. Positional argument validation
		{
			name: "Positional argument validation (no args)",
			setup: func(t *testing.T, dir string) []string {
				return []string{}
			},
			wantError:      true,
			wantErrorMatch: "accepts 2 arg(s), received 0",
		},
		{
			name: "Positional argument validation (1 arg)",
			setup: func(t *testing.T, dir string) []string {
				return []string{"one"}
			},
			wantError:      true,
			wantErrorMatch: "accepts 2 arg(s), received 1",
		},
		{
			name: "Positional argument validation (3 args)",
			setup: func(t *testing.T, dir string) []string {
				return []string{"one", "two", "three"}
			},
			wantError:      true,
			wantErrorMatch: "accepts 2 arg(s), received 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			args := tt.setup(t, dir)

			cmd := commandCheck()

			// We only want to capture out/err, not pollute real stdout
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(args)

			err := cmd.Execute()
			if (err != nil) != tt.wantError {
				t.Fatalf("commandCheck().Execute() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.wantError && tt.wantErrorMatch != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrorMatch)
				}
				if !strings.Contains(err.Error(), tt.wantErrorMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrorMatch)
				}
			}
		})
	}
}
