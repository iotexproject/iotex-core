// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a recovery tool that recovers a corrupted or missing state database.
// To use, run "make recover"
package main

import (
	"context"
	"flag"
	"fmt"
	glog "log"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state/factory"
)

// recoveryHeight is the blockchain height being recovered to
var recoveryHeight int

func init() {
	flag.IntVar(&recoveryHeight, "recovery-height", 0, "Recovery height")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: recover -config-path=[string]\n -recovery-height=[int]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	genesisCfg, err := genesis.New()
	if err != nil {
		glog.Fatalln("Failed to new genesis config.", zap.Error(err))
	}

	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}

	cfg.Genesis = genesisCfg

	log.S().Infof("Config in use: %+v", cfg)

	// create server
	svr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to create server.", zap.Error(err))
	}

	// recover chain and state
	bc := svr.ChainService(cfg.Chain.ID).Blockchain()
	sf := svr.ChainService(cfg.Chain.ID).StateFactory()
	if err := bc.Start(context.Background()); err == nil {
		log.L().Info("State DB status is normal.")
	}
	defer func() {
		if err := bc.Stop(context.Background()); err != nil {
			log.L().Fatal("Failed to stop blockchain")
		}
	}()
	if err := recoverChainAndState(bc, sf, cfg, uint64(recoveryHeight)); err != nil {
		log.L().Fatal("Failed to recover chain and state.", zap.Error(err))
	} else {
		log.S().Infof("Success to recover chain and state to target height %d", recoveryHeight)
	}
}

// recoverChainAndState recovers the chain to target height and refresh state db if necessary
func recoverChainAndState(bc blockchain.Blockchain, sf factory.Factory, cfg config.Config, targetHeight uint64) error {
	// recover the blockchain to target height(blockDAO)
	if err := bc.BlockDAO().DeleteBlockToTarget(targetHeight); err != nil {
		return errors.Wrapf(err, "failed to recover blockchain to target height %d", targetHeight)
	}
	stateHeight, err := sf.Height()
	if err != nil {
		return err
	}
	if targetHeight < stateHeight {
		// delete existing state DB (build from scratch)
		if fileutil.FileExists(cfg.Chain.TrieDBPath) && os.Remove(cfg.Chain.TrieDBPath) != nil {
			return errors.New("failed to delete existing state DB")
		}
	}
	return nil
}
