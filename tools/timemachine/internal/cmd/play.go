// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// play represents the play command
var play = &cobra.Command{
	Use:   "play [height]",
	Short: "Play blocks from chain.db to trie.db without committing the height's block.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		stopHeight, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}
		svr, err := newMiniServer(miniServerConfig(), WithStopHeight(stopHeight))
		if err != nil {
			return err
		}
		if err = svr.checkIndexer(); err != nil {
			return err
		}
		log.L().Info("play the height's block successfully.")
		return nil
	},
}
