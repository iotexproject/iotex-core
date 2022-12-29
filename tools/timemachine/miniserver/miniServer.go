// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package miniserver

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// const
const (
	// config files
	_configPath  = "./tools/timemachine/etc/config.yaml"
	_genesisPath = "./tools/timemachine/etc/genesis.yaml"
)

type (
	// MiniServer is an instance to build chain service
	MiniServer struct {
		cs         *chainservice.ChainService
		stopHeight uint64
	}

	// Option sets MiniServer construction parameter
	Option func(*MiniServer)
)

// WithStopHeightOption sets the stopHeight
func WithStopHeightOption(stopHeight uint64) Option {
	return func(svr *MiniServer) {
		svr.stopHeight = stopHeight
	}
}

// NewMiniServer creates instace and runs chainservice
func NewMiniServer(cfg config.Config,  opts ...Option) (*MiniServer, error) {
	svr := &MiniServer{}
	for _, opt := range opts {
		opt(svr)
	}

	builder := chainservice.NewBuilder(
		cfg,
		chainservice.WithStopHeightBuilderOption(svr.stopHeight),
	)
	cs, err := builder.SetP2PAgent(p2p.NewDummyAgent()).Build()
	if err != nil {
		return nil, err
	}
	svr.cs = cs

	ctx := Context(cfg)
	if err = svr.cs.BlockDAO().Start(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if err := svr.cs.BlockDAO().Stop(ctx); err != nil {
			log.S().Panic("failed to stop blockdao", zap.Error(err))
		}
	}()

	if err := svr.checkSanity(); err != nil {
		return nil, err
	}
	return svr, nil
}

// Config returns the config data from yaml
func Config() config.Config {
	if _, err := os.Stat(_genesisPath); errors.Is(err, os.ErrNotExist) {
		log.S().Fatalf("%s is not exist", _genesisPath)
	}
	genesisCfg, err := genesis.New(_genesisPath)
	if err != nil {
		log.S().Panic("failed to read genesis.yaml", zap.Error(err))
	}
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	block.LoadGenesisHash(&genesisCfg)
	if _, err := os.Stat(_configPath); errors.Is(err, os.ErrNotExist) {
		log.S().Fatalf("%s is not exist", _configPath)
	}
	cfg, err := config.New([]string{_configPath}, []string{})
	if err != nil {
		log.S().Panic("failed to read config.yaml", zap.Error(err))
	}
	cfg.Genesis = genesisCfg
	return cfg
}

// Context adds blockchain and genesis contexts
func Context(cfg config.Config) context.Context {
	ctx := genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			context.Background(),
			protocol.BlockchainCtx{
				ChainID: cfg.Chain.ID,
			},
		),
		cfg.Genesis,
	)
	return protocol.WithFeatureWithHeightCtx(ctx)
}

func (svr *MiniServer) checkSanity() error {
	daoHeight, err := svr.cs.BlockDAO().Height()
	if err != nil {
		return err
	}
	indexerHeight, err := svr.cs.StateFactory().Height()
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
