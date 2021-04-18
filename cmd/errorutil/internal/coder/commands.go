package coder

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/layer5io/meshkit/cmd/errorutil/internal/component"

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

func CommandAnalyze() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a directory tree",
		Long:  `analyze is for analyzing a directory tree for error codes`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool(verboseCmdFlag)
			rootDir, _ := cmd.Flags().GetString(rootDirCmdFlag)
			update := false
			updateAll := false
			config.Logging(verbose)
			errorsInfo := mesherr.NewErrorsInfo()
			err := walk(rootDir, update, updateAll, errorsInfo)
			if err != nil {
				log.Errorf("Unable to walk the walk: %v", err)
				return err
			}
			jsn, _ := json.MarshalIndent(errorsInfo, "", "  ")
			fname := filepath.Join(rootDir, config.App+"_analyze_errors.json")
			err = ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				log.Errorf("Unable to write to file %s (%v)", fname, err)
				return err
			}
			mesherr.Summarize(errorsInfo)
			componentInfo, err := component.New(rootDir)
			if err != nil {
				log.Fatalf("Unable to read component info file (%v)", err)
				return err
			}
			mesherr.Export(componentInfo, errorsInfo)
			return nil
		},
	}
}

func CommandUpdate() *cobra.Command {
	var updateAll bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update error codes and details",
		Long:  "update is for replacing error codes if necessary, and updating error details",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool(verboseCmdFlag)
			rootDir, _ := cmd.Flags().GetString(rootDirCmdFlag)
			update := true
			updateAll, _ := cmd.Flags().GetBool(updateAllCmdFlag)
			config.Logging(verbose)
			errorsInfo := mesherr.NewErrorsInfo()
			err := walk(rootDir, update, updateAll, errorsInfo)
			if err != nil {
				log.Errorf("Unable to walk the walk: %v", err)
			}
			jsn, _ := json.MarshalIndent(errorsInfo, "", "  ")
			fname := filepath.Join(rootDir, config.App+"_analyze_errors.json")
			err = ioutil.WriteFile(fname, jsn, 0600)
			if err != nil {
				log.Errorf("Unable to write to file %s (%v)", fname, err)
				return err
			}
			mesherr.Summarize(errorsInfo)
			componentInfo, err := component.New(rootDir)
			if err != nil {
				log.Fatalf("Unable to read component info file (%v)", err)
				return err
			}
			mesherr.Export(componentInfo, errorsInfo)
			return nil
		},
	}
	cmd.PersistentFlags().BoolVarP(&updateAll, updateAllCmdFlag, "a", false, "update all error codes and details (even ones where the error code has correct format")
	return cmd
}

func RootCommand() *cobra.Command {
	var verbose bool
	var rootDir string
	cmd := &cobra.Command{Use: config.App}
	cmd.PersistentFlags().BoolVarP(&verbose, verboseCmdFlag, "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&rootDir, rootDirCmdFlag, "d", ".", "root directory")
	cmd.AddCommand(CommandAnalyze())
	cmd.AddCommand(CommandUpdate())
	return cmd
}
