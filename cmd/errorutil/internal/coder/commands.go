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
	verboseCmdFlag   = "verbose"
	rootDirCmdFlag   = "dir"
	updateAllCmdFlag = "all"
)

func commandAnalyze() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze analyzes a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, err := cmd.Flags().GetBool(verboseCmdFlag)
			if err != nil {
				return err
			}
			rootDir, err := cmd.Flags().GetString(rootDirCmdFlag)
			if err != nil {
				return err
			}
			config.Logging(verbose)
			errorsInfo := mesherr.NewInfoAll()
			err = walkAnalyze(rootDir, errorsInfo)
			if err != nil {
				return err
			}
			jsn, err := json.MarshalIndent(errorsInfo, "", "  ")
			if err != nil {
				return err
			}
			fname := filepath.Join(rootDir, config.App+"_analyze_errors.json")
			err = ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				return err
			}
			err = mesherr.SummarizeAnalysis(errorsInfo, rootDir)
			if err != nil {
				return err
			}
			componentInfo, err := component.New(rootDir)
			if err != nil {
				return err
			}
			return mesherr.Export(componentInfo, errorsInfo, rootDir)
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
			verbose, err := cmd.Flags().GetBool(verboseCmdFlag)
			if err != nil {
				return err
			}
			rootDir, err := cmd.Flags().GetString(rootDirCmdFlag)
			if err != nil {
				return err
			}
			updateAll, err := cmd.Flags().GetBool(updateAllCmdFlag)
			if err != nil {
				return err
			}
			config.Logging(verbose)
			errorsInfo := mesherr.NewInfoAll()
			err = walkUpdate(rootDir, updateAll, errorsInfo)
			if err != nil {
				return err
			}
			jsn, err := json.MarshalIndent(errorsInfo, "", "  ")
			if err != nil {
				return err
			}
			fname := filepath.Join(rootDir, config.App+"_analyze_errors.json")
			err = ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				return err
			}
			err = mesherr.SummarizeAnalysis(errorsInfo, rootDir)
			if err != nil {
				return err
			}
			componentInfo, err := component.New(rootDir)
			if err != nil {
				return err
			}
			return mesherr.Export(componentInfo, errorsInfo, rootDir)
		},
	}
	cmd.PersistentFlags().BoolVarP(&updateAll, updateAllCmdFlag, "a", false, "update all error codes and details (including ones where the error code has correct format")
	return cmd
}

func RootCommand() *cobra.Command {
	var verbose bool
	var rootDir string
	cmd := &cobra.Command{Use: config.App}
	cmd.PersistentFlags().BoolVarP(&verbose, verboseCmdFlag, "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&rootDir, rootDirCmdFlag, "d", ".", "root directory")
	cmd.AddCommand(commandAnalyze())
	cmd.AddCommand(commandUpdate())
	return cmd
}
