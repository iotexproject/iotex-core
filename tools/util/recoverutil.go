// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

// RecoverChainAndState recovers the chain to target height and refresh state db if necessary
func RecoverChainAndState(bc blockchain.Blockchain, registry *protocol.Registry, cfg config.Config, targetHeight uint64) error {
	var buildStateFromScratch bool
	stateHeight, err := bc.Factory().Height()
	if err != nil {
		return err
	}
	if stateHeight == 0 {
		buildStateFromScratch = true
	}
	if targetHeight > 0 {
		if err := recoverToHeight(bc, targetHeight); err != nil {
			return errors.Wrapf(err, "failed to recover blockchain to target height %d", targetHeight)
		}
		if stateHeight > targetHeight {
			buildStateFromScratch = true
		}
	}

	if buildStateFromScratch {
		return refreshStateDB(bc, registry, cfg)
	}
	return nil
}

// recoverToHeight recovers the blockchain to target height
func recoverToHeight(bc blockchain.Blockchain, targetHeight uint64) error {
	tipHeight := bc.TipHeight()
	for tipHeight > targetHeight {
		if err := bc.BlockDAO().DeleteTipBlock(); err != nil {
			return err
		}
		tipHeight--
	}
	return nil
}

// refreshStateDB deletes the existing state DB and creates a new one with state changes from genesis block
func refreshStateDB(bc blockchain.Blockchain, registry *protocol.Registry, cfg config.Config) (err error) {
	// Delete existing state DB and reinitialize it
	sf := bc.Factory()
	if fileutil.FileExists(cfg.Chain.TrieDBPath) && os.Remove(cfg.Chain.TrieDBPath) != nil {
		return errors.New("failed to delete existing state DB")
	}
	if cfg.Chain.EnableTrielessStateDB {
		sf, err = factory.NewStateDB(cfg, factory.DefaultStateDBOption())
	} else {
		sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
	}
	if err != nil {
		return errors.Wrapf(err, "failed to reinitialize state DB")
	}
	for _, p := range registry.All() {
		sf.AddActionHandlers(p)
	}

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockTimeStamp: time.Unix(cfg.Genesis.Timestamp, 0),
		Registry:       registry,
	})
	if err = sf.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start state factory")
	}
	if err = sf.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop state factory")
	}
	return nil
}
