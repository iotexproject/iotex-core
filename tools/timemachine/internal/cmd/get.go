// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"

	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/tools/timemachine/miniserver"
)

// get represents the get command
var get = &cobra.Command{
	Use:   "get",
	Short: "Show current height of trie.db and chain.db",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		cfg := miniserver.Config(_configPath, _genesisPath)
		dbConfig := cfg.DB
		dbConfig.DbPath = cfg.Chain.ChainDBPath
		deser := block.NewDeserializer(cfg.Chain.EVMNetworkID)
		blkStore, err := filedao.NewFileDAO(dbConfig, deser)
		if err != nil {
			return err
		}
		ctx := context.Background()
		if err = blkStore.Start(ctx); err != nil {
			return err
		}
		defer blkStore.Stop(ctx)
		chainHeight, err := blkStore.Height()
		if err != nil {
			return err
		}

		triedao, err := db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
		if err != nil {
			return err
		}
		if err = triedao.Start(ctx); err != nil {
			return err
		}
		defer triedao.Stop(ctx)
		trieHeight, err := triedao.Get(factory.AccountKVNamespace, []byte(factory.CurrentHeightKey))
		if err != nil {
			return err
		}

		log.S().Infof("current chain.db height is %d", chainHeight)
		log.S().Infof("current trie.db height is %d", byteutil.BytesToUint64(trieHeight))
		return nil
	},
}
