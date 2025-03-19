package rolldpos

import (
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	// ChainManager defines the blockchain interface
	ChainManager interface {
		// BlockProposeTime return propose time by height
		BlockProposeTime(uint64) (time.Time, error)
		// BlockCommitTime return commit time by height
		BlockCommitTime(uint64) (time.Time, error)
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(timestamp time.Time) (*block.Block, error)
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(blk *block.Block) error
		// TipHeight returns tip block's height
		Tip() (uint64, hash.Hash256)
		// ChainAddress returns chain address on parent chain, the root chain return empty.
		ChainAddress() string
		// StateReader returns the state reader
		StateReader() (protocol.StateReader, error)
		// Fork creates a new chain manager with the given hash
		Fork(hash hash.Hash256) (ChainManager, error)
	}

	StateReaderFactory interface {
		StateReaderAt(*block.Header) (protocol.StateReader, error)
	}

	chainManager struct {
		bc  blockchain.Blockchain
		srf StateReaderFactory
	}
)

// NewChainManager creates a chain manager
func NewChainManager(bc blockchain.Blockchain, srf StateReaderFactory) ChainManager {
	return &chainManager{
		bc:  bc,
		srf: srf,
	}
}

// BlockProposeTime return propose time by height
func (cm *chainManager) BlockProposeTime(height uint64) (time.Time, error) {
	if height == 0 {
		return time.Unix(cm.bc.Genesis().Timestamp, 0), nil
	}
	header, err := cm.bc.BlockHeaderByHeight(height)
	if err != nil {
		return time.Time{}, errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return header.Timestamp(), nil
}

// BlockCommitTime return commit time by height
func (cm *chainManager) BlockCommitTime(height uint64) (time.Time, error) {
	footer, err := cm.bc.BlockFooterByHeight(height)
	if err != nil {
		return time.Time{}, errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return footer.CommitTime(), nil
}

// MintNewBlock creates a new block with given actions
func (cm *chainManager) MintNewBlock(timestamp time.Time) (*block.Block, error) {
	return cm.bc.MintNewBlock(timestamp)
}

// CommitBlock validates and appends a block to the chain
func (cm *chainManager) CommitBlock(blk *block.Block) error {
	return cm.bc.CommitBlock(blk)
}

// ValidateBlock validates a new block before adding it to the blockchain
func (cm *chainManager) ValidateBlock(blk *block.Block) error {
	return cm.bc.ValidateBlock(blk)
}

// TipHeight returns tip block's height
func (cm *chainManager) Tip() (uint64, hash.Hash256) {
	return cm.bc.TipHeight(), cm.bc.TipHash()
}

// ChainAddress returns chain address on parent chain, the root chain return empty.
func (cm *chainManager) ChainAddress() string {
	return cm.bc.ChainAddress()
}

// StateReader returns the state reader
func (cm *chainManager) StateReader() (protocol.StateReader, error) {
	header, err := cm.bc.BlockHeaderByHeight(cm.bc.TipHeight())
	if err != nil {
		return nil, err
	}
	return cm.srf.StateReaderAt(header)
}

// Fork creates a new chain manager with the given hash
func (cm *chainManager) Fork(hash hash.Hash256) (ChainManager, error) {
	fork, err := cm.bc.Fork(hash)
	if err != nil {
		return nil, err
	}
	return NewChainManager(fork, cm.srf), nil
}
