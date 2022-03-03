package cmd

import (
	"context"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/pkg/errors"
)

type miniServer struct {
	svr *itx.Server
	cfg config.Config
}

func NewMiniServer(cfg config.Config) *miniServer {
	// create server
	svr, err := itx.NewServer(cfg)
	if err != nil {
		panic(err)
	}

	dao := svr.ChainService(cfg.Chain.ID).BlockDAO()
	initCtx := prepareContext(cfg)
	err = dao.Start(initCtx)
	if err != nil {
		panic(err)
	}

	sf := svr.ChainService(cfg.Chain.ID).StateFactory()
	if err := sanityCheck(dao, sf); err != nil {
		panic(err)
	}
	return &miniServer{svr, cfg}
}

func (mini *miniServer) Context() context.Context {
	return prepareContext(mini.cfg)
}

func (mini *miniServer) BlockDao() blockdao.BlockDAO {
	return mini.svr.ChainService(mini.cfg.Chain.ID).BlockDAO()
}

func (mini *miniServer) Factory() factory.Factory {
	return mini.svr.ChainService(mini.cfg.Chain.ID).StateFactory()
}

func loadConfig() config.Config {
	genesisPath := "/home/haaai/iotex/iotex-core/tools/blockplayer/cmds/genesis.yaml"
	genesisCfg, err := genesis.New(genesisPath)
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	block.LoadGenesisHash(&genesisCfg)
	if err != nil {
		panic(err)
	}
	configPath := "/home/haaai/iotex/iotex-core/tools/blockplayer/cmds/config.yaml"
	cfg, err := config.New([]string{configPath}, []string{})
	if err != nil {
		panic(err)
	}
	cfg.Genesis = genesisCfg
	// Set the evmID
	config.SetEVMNetworkID(cfg.Chain.EVMNetworkID)
	return cfg
}

func prepareContext(cfg config.Config) context.Context {
	cc := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{ChainID: cfg.Chain.ID})
	cc2 := genesis.WithGenesisContext(cc, cfg.Genesis)
	return protocol.WithFeatureWithHeightCtx(cc2)
}

func sanityCheck(dao blockdao.BlockDAO, indexer blockdao.BlockIndexer) error {
	daoHeight, err := dao.Height()
	if err != nil {
		return err
	}
	indexerHeight, err := indexer.Height()
	if err != nil {
		return err
	}
	if indexerHeight > daoHeight {
		return errors.New("the height of indexer shouldn't be larger than the height of chainDB")
	}
	return nil
}
