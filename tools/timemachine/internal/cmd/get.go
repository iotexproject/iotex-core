// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/timemachine/minifactory"
	"github.com/iotexproject/iotex-core/tools/timemachine/miniserver"
)

// get represents the get command
var get = &cobra.Command{
	Use:   "get",
	Short: "Show current height of trie.db and chain.db",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		svr, err := miniserver.NewMiniServer(miniserver.Config(), minifactory.Get)
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
		log.S().Infof("current chain.db height is %d", daoHeight)
		log.S().Infof("current trie.db height is %d", indexerHeight)
		return nil
	},
}
