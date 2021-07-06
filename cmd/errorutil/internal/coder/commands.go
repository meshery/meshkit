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
This tool analyzes, verifies and updates error codes in Meshery source code trees. 
It extracts error details into a file that can be used for publishing all error code references on the Meshery website.

It is intended to be run locally and as part of a CI workflow.

- Errors names and codes are namespaced to components, i.e. they need to be unique within a component, which is verified by this tool.
- A component corresponds usually to a repository. Components have a type and a name. 
  They are also returned from the ComponentInfo endpoint, e.g. for adapters.
  Examples of a component types are 'adapter' and 'library', corresponding examples of names are 'istio' and 'meshkit'.
- There are no predefined error code ranges for components.
  Every component is free to use its own range, but it looks like the convention is to start at 1000.
- Errors are not to be reused across components and modules.
- Codes carry no meaning, as e.g. HTTP status codes do.
- In the code, create string var's or const's with names starting with Err[A-Z] and ending in Code, e.g. 'ErrApplyManifestCode'.
- Set the value to any string, like "replace_me" (no convention here), e.g. ErrApplyManifestCode = "test_code".
- If the value is a string, this tool will replace it with the next integer.
- If the value is an int, e.g. ErrGetName = "1000" the tool will not replace it unless it is forced (command line flag --force).
  If forced, all codes are renumbered. This can be useful to tidy up in earlier implementations of meshkit error codes.
- Setting an error code to a call expression like ErrNoneDatabase = errors.NewDefault(ErrNoneDatabaseCode, "No Database selected")
  is not allowed. This tool emits a warning if a call expression is detected.
- Using errors.NewDefault(...) is deprecated. This tool emits a warning if this is detected.
- Use errors.New(...) from meshkit to create actual errors with all the details.
  This is often done in a factory function. It is important that the error code variable is used here, not a literal.
  Specify detailed descriptions, probable causes, and remedies. They need to be string literals, call expressions are ignored.
  This tool extracts this information from the code and exports it.
- By convention, error codes and the factory functions live in files called error.go. The tool checks all files, but updates only error.go files.
- This tool will create a couple of files, one of them is designed to be used to generate the error reference on the meshery website.
  The file errorutil_analyze_summary.json contains a summary of the analysis, notably lists of duplicates etc.
- The tool requires a file called component_info.json. Its location can be customized, by default it is the root directory (-d flag). 
  This file has the following content, with concrete values specific for each component:
  {
    "name": "meshkit",
    "type": "library",
    "next_error_code": 11010
  }
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
