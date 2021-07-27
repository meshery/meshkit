package coder

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	mesherr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
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
	err = ioutil.WriteFile(fname, jsn, 0600)
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
			globalFlags, err := getGlobalFlags(cmd)
			if err != nil {
				return err
			}
			return walkSummarizeExport(globalFlags, false, false)
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
			globalFlags, err := getGlobalFlags(cmd)
			if err != nil {
				return err
			}
			updateAll, err := cmd.Flags().GetBool(forceUpdateAllCodesCmdFlag)
			if err != nil {
				return err
			}
			return walkSummarizeExport(globalFlags, true, updateAll)
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
- Error details defined using the errors.New(...) function from MeshKit.
 - The error code constant (or variable) has to be passed as first parameter, not a string literal.
 - Use string literals in the error details string array parameters. 
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
A CI workflow is used to replace the placeholder code strings with integer code, and export errors. Using this export, a PR is created 
by the workflow to update the error code reference documentation.

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
`)
		},
	}
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
	return cmd
}
