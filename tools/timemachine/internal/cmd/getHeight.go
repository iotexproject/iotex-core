// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
)

// getHeight represents the getheight command
var getHeight = &cobra.Command{
	Use:   "getheight",
	Short: "Show the tipheight of stateDB and chainDB",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svr, err := newMiniServer(miniServerConfig())
		if err != nil {
			return err
		}
		daoHeight, err := svr.BlockDao().Height()
		if err != nil {
			return err
		}
		indexerHeight, err := svr.Factory().Height()
		if err != nil {
			return err
		}
		cmd.Println("BlockDao's Height:", daoHeight)
		cmd.Println("Indexer's Height:", indexerHeight)
		return nil
	},
}
