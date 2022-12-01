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
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	miniServer struct {
		ctx         context.Context
		cfg         config.Config
		cs          *chainservice.ChainService
		commitBlock bool
		stopHeight  uint64
	}

	// Option sets miniServer construction parameter
	Option func(*miniServer)
)

// EnableCommitBlock enables to commit block
func EnableCommitBlock() Option {
	return func(svr *miniServer) {
		svr.commitBlock = true
	}
}

// WithStopHeight sets the stopHeight
func WithStopHeight(height uint64) Option {
	return func(svr *miniServer) {
		svr.stopHeight = height
	}
}

func newMiniServer(cfg config.Config, opts ...Option) (*miniServer, error) {
	builder := chainservice.NewBuilder(cfg)
	cs, err := builder.SetP2PAgent(p2p.NewDummyAgent()).Build()
	if err != nil {
		return nil, err
	}
	svr := &miniServer{
		cfg: cfg,
		cs:  cs,
	}
	for _, opt := range opts {
		opt(svr)
	}
	svr.ctx = svr.Context()
	if err = cs.BlockDAO().Start(svr.ctx); err != nil {
		return nil, err
	}
	if err := svr.checkSanity(); err != nil {
		return nil, err
	}
	return svr, nil
}

func miniServerConfig() config.Config {
	var (
		genesisPath = os.Getenv("IOTEX_HOME") + "/etc/genesis.yaml"
		configPath  = os.Getenv("IOTEX_HOME") + "/etc/config.yaml"
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

func (svr *miniServer) Context() context.Context {
	cfg := svr.cfg
	blockchainCtx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{ChainID: cfg.Chain.ID})
	genesisContext := genesis.WithGenesisContext(blockchainCtx, cfg.Genesis)
	featureContext := protocol.WithTestCtx(genesisContext, protocol.TestCtx{
		DisableCheckIndexer: true,
		CommitBlock:         svr.commitBlock,
		StopHeight:          svr.stopHeight,
	})
	return protocol.WithFeatureWithHeightCtx(featureContext)
}

func (svr *miniServer) BlockDao() blockdao.BlockDAO {
	return svr.cs.BlockDAO()
}

func (svr *miniServer) Factory() factory.Factory {
	return svr.cs.StateFactory()
}

func (svr *miniServer) checkSanity() error {
	daoHeight, err := svr.BlockDao().Height()
	if err != nil {
		return err
	}
	indexerHeight, err := svr.Factory().Height()
	if err != nil {
		return err
	}
	if indexerHeight > daoHeight {
		return errors.Errorf("the height of trie.db: %d shouldn't be larger than the height of chain.db: %d.", indexerHeight, daoHeight)
	}
	if svr.stopHeight > daoHeight {
		return errors.Errorf("the stopHeight: %d shouldn't be larger than the height of chain.db: %d.", svr.stopHeight, daoHeight)
	}
	if svr.stopHeight < indexerHeight {
		return errors.Errorf("the stopHeight: %d shouldn't be smaller than the current height of trie.db: %d.", svr.stopHeight, indexerHeight)
	}
	return nil
}

func (svr *miniServer) checkIndexer() error {
	checker := blockdao.NewBlockIndexerChecker(svr.BlockDao())
	if err := checker.CheckIndexer(svr.ctx, svr.Factory(), svr.stopHeight, func(height uint64) {
		if height%5000 == 0 {
			log.L().Info(
				"trie.db is catching up.",
				zap.Uint64("height", height),
			)
		}
	}); err != nil {
		return err
	}
	log.L().Info("trie.db is up to date.", zap.Uint64("height", svr.stopHeight))
	return nil
}
