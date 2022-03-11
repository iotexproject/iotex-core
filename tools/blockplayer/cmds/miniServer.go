package cmd

import (
	"context"
	"os"
	"path"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/pkg/errors"
)

type miniServer struct {
	svr *chainservice.ChainService
	cfg config.Config
}

func NewMiniServer(cfg config.Config) (*miniServer, error) {
	svr, err := chainservice.New(cfg, p2p.NewDummyAgent())
	if err != nil {
		return nil, err
	}
	miniSvr := &miniServer{svr, cfg}
	if err = svr.BlockDAO().Start(miniSvr.Context()); err != nil {
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
	return mini.svr.BlockDAO()
}

func (mini *miniServer) Factory() factory.Factory {
	return mini.svr.StateFactory()
}

func MiniServerConfig() config.Config {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	genesisPath := path.Join(pwd, "genesis.yaml")
	if _, err := os.Stat(genesisPath); errors.Is(err, os.ErrNotExist) {
		panic("please put genesis.yaml under current dir")
	}
	genesisCfg, err := genesis.New(genesisPath)
	if err != nil {
		panic(err)
	}
	genesis.SetGenesisTimestamp(genesisCfg.Timestamp)
	block.LoadGenesisHash(&genesisCfg)
	configPath := path.Join(pwd, "config.yaml")
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		panic("please put config.yaml under current dir")
	}
	cfg, err := config.New([]string{configPath}, []string{})
	if err != nil {
		panic(err)
	}
	cfg.Genesis = genesisCfg
	// Set the evmID
	config.SetEVMNetworkID(cfg.Chain.EVMNetworkID)
	return cfg
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
		return errors.New("the height of indexer shouldn't be larger than the height of chainDB")
	}
	return nil
}
