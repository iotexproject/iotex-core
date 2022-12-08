// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package miniserver

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
		ctx        context.Context
		cfg        config.Config
		cs         *chainservice.ChainService
		stopHeight uint64
	}

	// Option sets miniServer construction parameter
	Option func(*miniServer)
)

// WithStopHeightOption sets the stopHeight
func WithStopHeightOption(stopHeight uint64) Option {
	return func(svr *miniServer) {
		svr.stopHeight = stopHeight
	}
}

// NewMiniServer creates instace and runs chainservice
func NewMiniServer(cfg config.Config, operation int, opts ...Option) (*miniServer, error) {
	svr := &miniServer{
		cfg: cfg,
	}
	for _, opt := range opts {
		opt(svr)
	}

	builder := chainservice.NewBuilder(cfg,
		chainservice.WithOpTimeMachineBuilderOption(operation),
		chainservice.WithStopHeightBuilderOption(svr.stopHeight),
	)
	cs, err := builder.SetP2PAgent(p2p.NewDummyAgent()).Build()
	if err != nil {
		return nil, err
	}
	svr.cs = cs

	svr.ctx = svr.Context()
	if err = svr.cs.BlockDAO().Start(svr.ctx); err != nil {
		return nil, err
	}
	if err := svr.checkSanity(); err != nil {
		return nil, err
	}
	return svr, nil
}

// MiniServerConfig returns the config data from yaml
func MiniServerConfig() config.Config {
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
	ctx := genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			context.Background(),
			protocol.BlockchainCtx{
				ChainID: svr.cfg.Chain.ID,
			},
		),
		svr.cfg.Genesis,
	)
	return protocol.WithFeatureWithHeightCtx(ctx)
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
	return nil
}

func (svr *miniServer) CheckIndexer() error {
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
