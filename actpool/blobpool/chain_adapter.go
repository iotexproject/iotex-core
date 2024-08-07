package blobpool

import (
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type chainAdapter struct {
	g   *genesis.Genesis
	dao blockdao.BlockDAO
	bc  blockchain.Blockchain
}

func NewChainAdapter(g *genesis.Genesis, dao blockdao.BlockDAO) BlockChain {
	return &chainAdapter{g: g, dao: dao}
}

func (c *chainAdapter) Config() *genesis.Genesis {
	return c.g
}

func (c *chainAdapter) CurrentBlock() *block.Header {
	height, err := c.dao.Height()
	if err != nil {
		log.L().Error("failed to get height", zap.Error(err))
		return nil
	}
	blk, err := c.dao.HeaderByHeight(height)
	if err != nil {
		log.L().Error("failed to get block by height", zap.Error(err))
		return nil
	}
	return blk
}

func (c *chainAdapter) GetBlock(number uint64) *block.Block {
	blk, err := c.dao.GetBlockByHeight(number)
	if err != nil {
		log.L().Error("failed to get block by height", zap.Error(err))
		return nil
	}
	return blk
}

func (c *chainAdapter) EvmNetworkID() uint32 {
	return c.bc.EvmNetworkID()
}
