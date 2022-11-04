// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"
	"os"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/state/factory"
)

type miniServer struct {
	cs  *chainservice.ChainService
	cfg config.Config
}

func newMiniServer(cfg config.Config) (*miniServer, error) {
	builder := chainservice.NewBuilder(cfg)
	cs, err := builder.SetP2PAgent(p2p.NewDummyAgent()).Build()
	if err != nil {
		return nil, err
	}
	miniSvr := &miniServer{cs, cfg}
	if err = cs.BlockDAO().Start(miniSvr.Context()); err != nil {
		return nil, err
	}
	if err := miniSvr.checkSanity(); err != nil {
		return nil, err
	}
	return miniSvr, nil
}

func (mini *miniServer) Context() context.Context {
	cfg := mini.cfg
	blockchainCtx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{ChainID: cfg.Chain.ID})
	genesisContext := genesis.WithGenesisContext(blockchainCtx, cfg.Genesis)
	featureContext := protocol.WithTestCtx(genesisContext, protocol.TestCtx{DisableBlockDaoSync: true})
	return protocol.WithFeatureWithHeightCtx(featureContext)
}

func (mini *miniServer) BlockDao() blockdao.BlockDAO {
	return mini.cs.BlockDAO()
}

func (mini *miniServer) Factory() factory.Factory {
	return mini.cs.StateFactory()
}

func miniServerConfig() config.Config {
	var (
		genesisPath = "./genesis.yaml"
		configPath  = "./config.yaml"
		// genesisPath = "./tools/timemachine/genesis.yaml"
		// configPath  = "./tools/timemachine/config.yaml"
	)
	if _, err := os.Stat(genesisPath); errors.Is(err, os.ErrNotExist) {
		panic("please put genesis.yaml under current dir")
	}
	genesisCfg, err := genesis.New(genesisPath)
	if err != nil {
		panic(err)
	}
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	block.LoadGenesisHash(&genesisCfg)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		panic("please put config.yaml under current dir")
	}
	cfg, err := config.New([]string{configPath}, []string{})
	if err != nil {
		panic(err)
	}
	cfg.Genesis = genesisCfg
	return cfg
}

func (mini *miniServer) checkSanity() error {
	daoHeight, err := mini.BlockDao().Height()
	if err != nil {
		return err
	}
	indexerHeight, err := mini.Factory().Height()
	if err != nil {
		return err
	}
	if indexerHeight > daoHeight {
		return errors.New("the height of indexer shouldn't be larger than the height of chainDB")
	}
	return nil
}
