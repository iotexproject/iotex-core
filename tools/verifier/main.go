package main

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/tools/verifier/archive"
)

var rootCmd = &cobra.Command{
	Use: "verifier",
}

func main() {
	rootCmd.AddCommand(archive.CmdVerifyArchive)
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
