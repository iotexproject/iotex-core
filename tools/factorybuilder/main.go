// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
)

var command = &cobra.Command{
	Use:   "genesis.yaml config.yaml targetHeight [trie.db]",
	Short: "build trie.db based on genesis.yaml and config.yaml",
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		gs, err := genesis.New(args[0])
		if err != nil {
			return errors.Wrap(err, "failed to load genesis config")
		}
		cfg, err := config.New([]string{args[1]}, nil)
		if err != nil {
			return errors.Wrap(err, "failed to load config")
		}
		cfg.Genesis = gs
		targetHeight, ok := big.NewInt(0).SetString(args[2], 10)
		if !ok || !targetHeight.IsUint64() {
			return errors.Errorf("invalid target height %s", args[2])
		}
		if len(args) > 3 {
			cfg.Chain.TrieDBPath = args[3]
		}
		config.SetEVMNetworkID(cfg.Chain.EVMNetworkID)
		daoCfg := cfg.DB
		daoCfg.DbPath = cfg.Chain.ChainDBPath
		daoCfg.ReadOnly = true
		dao := blockdao.NewBlockDAO(nil, daoCfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
		builder := chainservice.NewBuilder(cfg).
			SetBlockDAO(dao).
			SetBlockSync(blocksync.NewDummyBlockSyncer())
		cs, err := builder.Build()
		if err != nil {
			return errors.Wrap(err, "failed to build chain service")
		}
		ctx := protocol.WithFeatureWithHeightCtx(
			genesis.WithGenesisContext(
				protocol.WithBlockchainCtx(
					context.Background(),
					protocol.BlockchainCtx{
						ChainID: cs.ChainID(),
					},
				),
				cfg.Genesis,
			),
		)
		checker := blockdao.NewBlockIndexerChecker(dao)
		if err := dao.Start(ctx); err != nil {
			return errors.Wrap(err, "failed to start block dao")
		}
		defer dao.Stop(ctx)
		factory := cs.StateFactory()
		if err := factory.Start(ctx); err != nil {
			return errors.Wrap(err, "failed to start state factory")
		}
		defer factory.Stop(ctx)
		return checker.CheckIndexer(ctx, cs.StateFactory(), targetHeight.Uint64(), func(height uint64) {
			if height%5000 == 0 {
				fmt.Printf("catching up to %d at %s\n", height, time.Now())
			}
		})
	},
}

func main() {
	if err := command.Execute(); err != nil {
		fmt.Printf("%+v\n", err)
	}
}
