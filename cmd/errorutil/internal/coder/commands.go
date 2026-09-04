package coder

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"

	"github.com/meshery/meshkit/cmd/errorutil/internal/config"
	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
	"github.com/spf13/cobra"
)

const (
	verboseCmdFlag             = "verbose"
	rootDirCmdFlag             = "dir"
	skipDirsCmdFlag            = "skip-dirs"
	outDirCmdFlag              = "out-dir"
	infoDirCmdFlag             = "info-dir"
	forceUpdateAllCodesCmdFlag = "force"
)

type globalFlags struct {
	verbose                  bool
	rootDir, outDir, infoDir string
	skipDirs                 []string
}

func defaultIfEmpty(value, defaultValue string) string {
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func getGlobalFlags(cmd *cobra.Command) (globalFlags, error) {
	flags := globalFlags{}
	verbose, err := cmd.Flags().GetBool(verboseCmdFlag)
	if err != nil {
		return flags, err
	}
	flags.verbose = verbose
	rootDir, err := cmd.Flags().GetString(rootDirCmdFlag)
	if err != nil {
		return flags, err
	}
	flags.rootDir = rootDir
	skipDirs, err := cmd.Flags().GetStringSlice(skipDirsCmdFlag)
	if err != nil {
		return flags, err
	}
	flags.skipDirs = skipDirs
	outDir, err := cmd.Flags().GetString(outDirCmdFlag)
	if err != nil {
		return flags, err
	}
	flags.outDir = defaultIfEmpty(outDir, rootDir) // if outDir is an empty string, rootDir is the default value
	infoDir, err := cmd.Flags().GetString(infoDirCmdFlag)
	if err != nil {
		return flags, err
	}
	flags.infoDir = defaultIfEmpty(infoDir, rootDir) // if infoDir is an empty string, rootDir is the default value
	return flags, nil
}

func walkSummarizeExport(globalFlags globalFlags, update bool, updateAll bool) error {
	config.Logging(globalFlags.verbose)
	errorsInfo := mesherr.NewInfoAll()
	err := walk(globalFlags, update, updateAll, errorsInfo)
	if err != nil {
		return err
	}
	// if it was an update, carry out a second pass to get latest state
	if update {
		errorsInfo = mesherr.NewInfoAll()
		err = walk(globalFlags, false, false, errorsInfo)
		if err != nil {
			return err
		}
	}
	jsn, err := json.MarshalIndent(errorsInfo, "", "  ")
	if err != nil {
		return err
	}
	fname := filepath.Join(globalFlags.outDir, config.App+"_analyze_errors.json")
	err = os.WriteFile(fname, jsn, 0600)
	if err != nil {
		return err
	}
	componentInfo, err := component.New(globalFlags.infoDir)
	if err != nil {
		return err
	}
	err = mesherr.SummarizeAnalysis(componentInfo, errorsInfo, globalFlags.outDir)
	if err != nil {
		return err
	}
	return mesherr.Export(componentInfo, errorsInfo, globalFlags.outDir)
}

func commandAnalyze() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze analyzes a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			gFlags, err := getGlobalFlags(cmd)
			if err != nil {
				return err
			}
			return walkSummarizeExport(gFlags, false, false)
		},
	}
}

func commandUpdate() *cobra.Command {
	var updateAll bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update error codes and details",
		Long:  "update replaces error codes where specified, and updates error details",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			gFlags, err := getGlobalFlags(cmd)
			if err != nil {
				return err
			}
			updateAll, err = cmd.Flags().GetBool(forceUpdateAllCodesCmdFlag)
			if err != nil {
				return err
			}
			return walkSummarizeExport(gFlags, true, updateAll)
		},
	}
	cmd.PersistentFlags().BoolVar(&updateAll, forceUpdateAllCodesCmdFlag, false, "Update and re-sequence all error codes.")
	return cmd
}

func commandDoc() *cobra.Command {
	return &cobra.Command{
		Use:   "doc",
		Short: "Print the documentation",
		Long:  "Print the documentation",
		Run: func(cmd *cobra.Command, args []string) {
			println(`
This tool analyzes, verifies and updates MeshKit compatible errors in Meshery Go source code trees.

A MeshKit compatible error consist of
- An error code defined as a constant or variable (preferably constant), of type string.
  - The naming convention for these variables is the regex "^Err[A-Z].+Code$", e.g. ErrApplyManifestCode.
  - The initial value of the code is a placeholder string, e.g. "replace_me", set by the developer.
  - The final value of the code is an integer, set by this tool, as part of a CI workflow.
- Error details defined using the function errors.New(code, severity, sdescription, ldescription, probablecause, remedy) from MeshKit.
 - The first parameter, 'code', has to be passed as the error code constant (or variable), not a string literal.
 - The second parameter, 'severity', has its own type; consult its Go-doc for further details.
 - The remaining parameters are string arrays for short and long description, probable cause, and suggested remediation.
 - Use string literals in these string arrays, not constants or variables, for any static texts.
 - Capitalize the first letter of each statement.
 - Call expressions can be used but will be ignored by the tool when exporting error details for the documentation.
 - Do not concatenate strings using the '+' operator, just add multiple elements to the string array.

Additionally, the following conventions apply:
- Errors are defined in each package, in a file named error.go
- Errors are namespaced to components, i.e. they need to be unique within a component (see below).
- Errors are not to be reused across components and modules.
- There are no predefined error code ranges for components. Every component is free to use its own range.
- Codes carry no meaning, as e.g. HTTP status codes do.

This tool produces three files:
- errorutil_analyze_errors.json: raw data with all errors and some metadata
- errorutil_analyze_summary.json: summary of raw data, also used for validation and troubleshooting
- errorutil_errors_export.json: export of errors which can be used to create the error code reference on the Meshery website

Typically, the 'analyze' command of the tool is used by the developer to verify errors, i.e. that there are no duplicate names or details.
A CI workflow is used to replace the placeholder code strings with integer code, and export errors. Using this export, the workflow updates
the error code reference documentation in the Meshery repository.

Meshery components and this tool:
- Meshery components have a name and a type.
- An example of a component is MeshKit with 'meshkit' as name, and 'library' as type.
- Often, a specific component corresponds to one git repository.
- The tool requires a file called component_info.json.
  This file has the following content, with concrete values specific for each component:
  {
    "name": "meshkit",
    "type": "library",
    "next_error_code": 1014
  }
- next_error_code is the value used by the tool to replace the error code placeholder string with the next integer.
- The tool updates next_error_code.

Both "replace_me" and locally-running 'errorutil update' are valid ways to arrive at an integer code.
Note a precondition for the check command: baseline must be the analysis of current's merge-base (the commit this branch forked from), NOT the current tip of the base branch. This tool does not resolve two independent branches allocating the same code from the same baseline — that is handled by the post-merge allocation process re-encountering the collision, not by this check.
`)
		},
	}
}

// ValidateNewErrors rejects newly introduced integer error codes that are
// not backed by a legitimate local allocation.
//
// Precondition: baseline must be the analysis of current's merge-base (the
// commit this branch forked from), NOT the current tip of the base branch.
// The allocation-counter checks (R2, R4) assume baseline is an ancestor of
// current; if baseline's own next_error_code has advanced past current's
// (e.g. because the base branch's post-merge allocation bot ran on
// unrelated PRs after this branch diverged), R2 reports a false counter
// regression. Callers wiring this into CI must construct baseline from the
// PR's merge-base, not from the base branch's tip at CI-run time.
func ValidateNewErrors(baseline, current mesherr.InfoAll, baselineNext, currentNext int, out io.Writer) error {
	baselineCodes := make(map[string]bool) // normalized numeric key
	baselineCounts := make(map[string]int) // normalized numeric key -> baseline occurrence count
	for _, entry := range baseline.Entries {
		if !entry.CodeIsInt {
			continue
		}
		n, err := strconv.Atoi(entry.Code)
		if err != nil {
			continue // defensive; CodeIsInt already guarantees this parses
		}
		key := strconv.Itoa(n)
		baselineCodes[key] = true
		baselineCounts[key]++
	}

	hasError := false

	// R1: a code's occurrence count in `current` must not exceed its baseline
	// count. Baseline debt (a code already duplicated pre-PR) is tolerated as
	// long as it doesn't grow; any growth - including a duplicated code gaining
	// a THIRD user - is a newly introduced collision and must fail.
	seen := make(map[string][]string)
	for _, entry := range current.Entries {
		if !entry.CodeIsInt {
			continue
		}
		n, err := strconv.Atoi(entry.Code)
		if err != nil {
			continue
		}
		key := strconv.Itoa(n)
		seen[key] = append(seen[key], entry.Name)
	}

	var duplicateCodes []string
	for code, names := range seen {
		if len(names) > 1 && len(names) > baselineCounts[code] {
			duplicateCodes = append(duplicateCodes, code)
		}
	}
	sort.Slice(duplicateCodes, func(i, j int) bool {
		a, _ := strconv.Atoi(duplicateCodes[i])
		b, _ := strconv.Atoi(duplicateCodes[j])
		return a < b
	})

	for _, code := range duplicateCodes {
		fmt.Fprintf(out, "Error: duplicate error code %q used by %v (was %d occurrence(s) in baseline, now %d)\n",
			code, seen[code], baselineCounts[code], len(seen[code]))
		hasError = true
	}

	// R2: the allocation counter must never appear to move backwards from
	// baseline to current. This is only a sound check when baseline is the
	// analysis of current's merge-base (fork point) with the base branch, NOT
	// the base branch's current tip - see the precondition documented on
	// ValidateNewErrors below.
	if baselineNext > 0 && currentNext > 0 && currentNext < baselineNext {
		fmt.Fprintf(out, "Error: allocation counter went backwards (baseline next_error_code: %d, current next_error_code: %d). "+
			"This usually means the baseline analysis is not the merge-base of this branch - "+
			"if the base branch's own error-code counter has advanced since this branch diverged "+
			"(e.g. post-merge allocation on unrelated PRs), re-analyze against the merge-base, not the base branch tip.\n",
			baselineNext, currentNext)
		hasError = true
	}

	for _, entry := range current.Entries {
		if !entry.CodeIsInt {
			continue
		}
		codeInt, err := strconv.Atoi(entry.Code)
		if err != nil {
			continue // CodeIsInt guarantees this parses; defensive only
		}
		key := strconv.Itoa(codeInt)

		// R3: identify newly introduced integer codes BY CODE, NOT NAME, so a
		// rename or a package move of an existing code is never treated as new.
		if baselineCodes[key] {
			continue
		}

		// R4: a genuinely new integer code is only valid if it falls inside the
		// allocation window errorutil update actually produced.
		if baselineNext > 0 && currentNext > 0 {
			if codeInt >= baselineNext && codeInt < currentNext {
				continue // legitimate local allocation
			}
			if baselineNext == currentNext {
				fmt.Fprintf(out, "Error: %s (%s) uses integer code %q; the allocation counter did not advance between baseline and current, so no new integer code can be valid - use \"replace_me\", or run `errorutil update` to allocate it\n",
					entry.Name, entry.Path, entry.Code)
			} else {
				fmt.Fprintf(out, "Error: %s (%s) uses integer code %q which is not present in the baseline and not within the recorded local allocation range [%d, %d); use \"replace_me\", or run `errorutil update` to allocate it\n",
					entry.Name, entry.Path, entry.Code, baselineNext, currentNext)
			}
			hasError = true
			continue
		}

		// No allocation metadata supplied: fall back to strict mode.
		fmt.Fprintf(out, "Error: %s (%s) uses integer code %q; use \"replace_me\" and let errorutil allocate the code\n",
			entry.Name, entry.Path, entry.Code)
		hasError = true
	}

	if hasError {
		return fmt.Errorf("error code validation failed; see details above")
	}
	return nil
}

func readSummaryNextCode(path, label string) (int, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s summary: %w", label, err)
	}
	var sum struct {
		NextCode int `json:"next_code"`
	}
	if err := json.Unmarshal(bytes, &sum); err != nil {
		return 0, fmt.Errorf("failed to parse %s summary: %w", label, err)
	}
	if sum.NextCode <= 0 {
		return 0, fmt.Errorf("%s: next_code missing or zero — expected an errorutil_analyze_summary.json", path)
	}
	return sum.NextCode, nil
}

func commandCheck() *cobra.Command {
	var baselineSummaryPath, currentSummaryPath string

	cmd := &cobra.Command{
		Use:          "check [baseline JSON] [current JSON]",
		Short:        "Checks that newly introduced error codes use a placeholder (e.g. replace_me)",
		Long:         `check compares the errors from the two provided JSON files (baseline and current) and ensures that any newly introduced error code does not use a manually assigned integer code, but rather a placeholder string. It validates local allocations if summary files are provided. Precondition: baseline must be the analysis of current's merge-base (the commit this branch forked from), NOT the current tip of the base branch.`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if (baselineSummaryPath != "" && currentSummaryPath == "") || (baselineSummaryPath == "" && currentSummaryPath != "") {
				return fmt.Errorf("both --baseline-summary and --current-summary must be provided if one is provided")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			baselineBytes, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			currentBytes, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}
			var baseline mesherr.InfoAll
			if err := json.Unmarshal(baselineBytes, &baseline); err != nil {
				return err
			}
			var current mesherr.InfoAll
			if err := json.Unmarshal(currentBytes, &current); err != nil {
				return err
			}

			baselineNext := -1
			currentNext := -1

			if baselineSummaryPath != "" && currentSummaryPath != "" {
				var err error
				baselineNext, err = readSummaryNextCode(baselineSummaryPath, "baseline")
				if err != nil {
					return err
				}
				currentNext, err = readSummaryNextCode(currentSummaryPath, "current")
				if err != nil {
					return err
				}
			}

			return ValidateNewErrors(baseline, current, baselineNext, currentNext, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&baselineSummaryPath, "baseline-summary", "", "path to baseline errorutil_analyze_summary.json")
	cmd.Flags().StringVar(&currentSummaryPath, "current-summary", "", "path to current errorutil_analyze_summary.json")
	return cmd
}

func RootCommand() *cobra.Command {
	cmd := &cobra.Command{Use: config.App}
	cmd.PersistentFlags().BoolP(verboseCmdFlag, "v", false, "verbose output")
	cmd.PersistentFlags().StringP(rootDirCmdFlag, "d", ".", "root directory")
	cmd.PersistentFlags().StringP(outDirCmdFlag, "o", "", "output directory")
	cmd.PersistentFlags().StringP(infoDirCmdFlag, "i", "", "directory containing the component_info.json file")
	cmd.PersistentFlags().StringSlice(skipDirsCmdFlag, []string{}, "directories to skip (comma-separated list, repeatable argument)")
	cmd.AddCommand(commandAnalyze())
	cmd.AddCommand(commandUpdate())
	cmd.AddCommand(commandDoc())
	cmd.AddCommand(commandCheck())
	return cmd
}
