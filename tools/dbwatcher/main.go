package main

import (
	"os"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "dbwatcher",
	Short: "Command-line interface for watch of IoTeX blockchain db file",
	Long:  "dbwatcher is a command-line interface for watch of IoTeX blockchain db file.",
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.AddCommand(WatchErigon)

	RootCmd.HelpFunc()
}

func main() {
	Execute()
}
