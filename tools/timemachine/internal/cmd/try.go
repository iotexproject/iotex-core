// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/timemachine/miniserver"
)

// try represents the try command
var try = &cobra.Command{
	Use:   "try [height]",
	Short: "Play blocks from chain.db to trie.db uncommitted stopHeight block.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		stopHeight, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}
		if _, err = miniserver.NewMiniServer(miniserver.Config(_configPath, _genesisPath), miniserver.WithStopHeightOption(stopHeight)); err != nil {
			log.S().Info(err.Error())
		}
		return nil
	},
}
