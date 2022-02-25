package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// This Var is useless currently
	GetHeight = &cobra.Command{
		Use:   "getheight",
		Short: "Show the tipheight of stateDB and chainDB",
		RunE: func(cmd *cobra.Command, args []string) error {
			svr := NewMiniServer(loadConfig())
			daoHeight, err := svr.BlockDao().Height()
			if err != nil {
				return err
			}
			indexerHeight, err := svr.Factory().Height()
			if err != nil {
				return err
			}
			c := color.New(color.FgRed).Add(color.Bold)
			c.Println("Indexer's Height:", indexerHeight)
			c.Println("BlockDao's Height:", daoHeight)
			return nil
		},
	}
)
