package coder

import (
	"encoding/json"
	"io/ioutil"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/config"
	mesherr "github.com/layer5io/meshkit/cmd/errorutil/internal/error"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	verboseCmdFlag   = "verbose"
	rootDirCmdFlag   = "dir"
	updateAllCmdFlag = "all"
)

func CommandAnalyze(errorsInfo *mesherr.ErrorsInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze is for analyzing a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool(verboseCmdFlag)
			rootDir, _ := cmd.Flags().GetString(rootDirCmdFlag)
			update := false
			updateAll := false
			config.Logging(verbose)
			walk(rootDir, update, updateAll, errorsInfo)
			jsn, _ := json.MarshalIndent(errorsInfo, "", "  ")
			fname := config.App + "_analyze_errors.json"
			err := ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				log.Errorf("Unable to write to file %s (%v)", fname, err)
			}
		},
	}
}

func CommandUpdate(errorsInfo *mesherr.ErrorsInfo) *cobra.Command {
	var updateAll bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update error codes and details",
		Long:  "update is for replacing error codes if necessary, and updating error details",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool(verboseCmdFlag)
			rootDir, _ := cmd.Flags().GetString(rootDirCmdFlag)
			update := true
			updateAll, _ := cmd.Flags().GetBool(updateAllCmdFlag)
			config.Logging(verbose)
			walk(rootDir, update, updateAll, errorsInfo)
			jsn, _ := json.MarshalIndent(errorsInfo, "", "  ")
			fname := config.App + "_analyze_errors.json"
			err := ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				log.Errorf("Unable to write to file %s (%v)", fname, err)
			}
		},
	}
	cmd.PersistentFlags().BoolVarP(&updateAll, updateAllCmdFlag, "a", false, "update all error codes and details (even ones where the error code has correct format")
	return cmd
}

func RootCommand(errorsInfo *mesherr.ErrorsInfo) *cobra.Command {
	var verbose bool
	var rootDir string
	cmd := &cobra.Command{Use: config.App}
	cmd.PersistentFlags().BoolVarP(&verbose, verboseCmdFlag, "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&rootDir, rootDirCmdFlag, "d", ".", "root directory")
	cmd.AddCommand(CommandAnalyze(errorsInfo))
	cmd.AddCommand(CommandUpdate(errorsInfo))
	return cmd
}
