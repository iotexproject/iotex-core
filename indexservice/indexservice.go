package indexservice

import (
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/cloudrds"
	"github.com/iotexproject/iotex-core/config"
)

// IndexService handle the index build for blocks
type IndexService struct {
	cfg config.IndexService
	rds cloudrds.RDSStore
}

// BuildIndex build the index for a block
func (idx *IndexService) BuildIndex(blk *blockchain.Block) error {
	return nil
}
