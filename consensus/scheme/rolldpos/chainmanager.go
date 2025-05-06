package rolldpos

import (
	"context"
	"strconv"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
)

var (
	blockMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_blockmint_metrics",
			Help: "Block mint metrics.",
		},
		[]string{"type"},
	)
)

type (
	// ChainManager defines the blockchain interface
	ChainManager interface {
		ForkChain
		// Start starts the chain manager
		Start(ctx context.Context) error
		// Fork creates a new chain manager with the given hash
		Fork(hash hash.Hash256) (ForkChain, error)
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(blk *block.Block) error
	}
	// ForkChain defines the blockchain interface
	ForkChain interface {
		// BlockProposeTime return propose time by height
		BlockProposeTime(uint64) (time.Time, error)
		// BlockCommitTime return commit time by height
		BlockCommitTime(uint64) (time.Time, error)
		// TipHeight returns tip block's height
		TipHeight() uint64
		// TipHash returns tip block's hash
		TipHash() hash.Hash256
		// StateReader returns the state reader
		StateReader() (protocol.StateReader, error)
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(time.Time, crypto.PrivateKey, hash.Hash256) (*block.Block, error)
	}
	// StateReaderFactory is the factory interface of state reader
	StateReaderFactory interface {
		StateReaderAt(uint64, hash.Hash256) (protocol.StateReader, error)
	}

	// BlockBuilderFactory is the factory interface of block builder
	BlockBuilderFactory interface {
		Mint(ctx context.Context, pk crypto.PrivateKey) (*block.Block, error)
		ReceiveBlock(*block.Block) error
	}

	chainManager struct {
		srf          StateReaderFactory
		timerFactory *prometheustimer.TimerFactory
		bc           blockchain.Blockchain
		bbf          BlockBuilderFactory
		pool         *proposalPool
	}

	forkChain struct {
		cm   *chainManager
		head *block.Header
		sr   protocol.StateReader
	}
)

func init() {
	prometheus.MustRegister(blockMtc)
}

func newForkChain(cm *chainManager, head *block.Header, sr protocol.StateReader) *forkChain {
	return &forkChain{
		cm:   cm,
		head: head,
		sr:   sr,
	}
}

// NewChainManager creates a chain manager
func NewChainManager(bc blockchain.Blockchain, srf StateReaderFactory, bbf BlockBuilderFactory) ChainManager {
	timerFactory, err := prometheustimer.New(
		"iotex_blockchain_perf",
		"Performance of blockchain module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(bc.ChainID()), 10)},
	)
	if err != nil {
		log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	}

	return &chainManager{
		bc:           bc,
		bbf:          bbf,
		timerFactory: timerFactory,
		srf:          srf,
		pool:         newProposalPool(),
	}
}

// BlockProposeTime return propose time by height
func (fc *forkChain) BlockProposeTime(height uint64) (time.Time, error) {
	t, err := fc.cm.BlockProposeTime(height)
	switch errors.Cause(err) {
	case nil:
		return t, nil
	case db.ErrNotExist:
		header, err := fc.cm.header(height, fc.tipHash())
		if err != nil {
			return time.Time{}, err
		}
		return header.Timestamp(), nil
	default:
		return time.Time{}, err
	}
}

func (cm *chainManager) BlockProposeTime(height uint64) (time.Time, error) {
	if height == 0 {
		return time.Unix(cm.bc.Genesis().Timestamp, 0), nil
	}
	head, err := cm.bc.BlockHeaderByHeight(height)
	if err != nil {
		return time.Time{}, errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return head.Timestamp(), nil
}

// BlockCommitTime return commit time by height
func (fc *forkChain) BlockCommitTime(height uint64) (time.Time, error) {
	return fc.cm.BlockCommitTime(height)
}

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

func (fc *forkChain) MintNewBlock(timestamp time.Time, pk crypto.PrivateKey, prevHash hash.Hash256) (*block.Block, error) {
	return fc.cm.mintNewBlock(timestamp, pk, prevHash, fc.tipInfo())
}

func (cm *chainManager) MintNewBlock(timestamp time.Time, pk crypto.PrivateKey, prevHash hash.Hash256) (*block.Block, error) {
	return cm.mintNewBlock(timestamp, pk, prevHash, cm.tipInfo())
}

func (cm *chainManager) mintNewBlock(timestamp time.Time, pk crypto.PrivateKey, prevHash hash.Hash256, tipInfo *protocol.TipInfo) (*block.Block, error) {
	mintNewBlockTimer := cm.timerFactory.NewTimer("MintBlock")
	defer mintNewBlockTimer.End()
	var (
		newblockHeight = tipInfo.Height + 1
		producer       address.Address
		err            error
	)
	// safety check
	if prevHash != tipInfo.Hash {
		return nil, errors.Errorf("invalid prev hash, expecting %x, got %x", prevHash, tipInfo.Hash)
	}
	producer = pk.PublicKey().Address()
	ctx := cm.mintContext(context.Background(), timestamp, producer, tipInfo)
	// create a new block
	log.L().Debug("Produce a new block.", zap.Uint64("height", newblockHeight), zap.Time("timestamp", timestamp), log.Hex("prevHash", prevHash[:]))
	// run execution and update state trie root hash
	blk, err := cm.bbf.Mint(ctx, pk)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}
	if err = cm.pool.AddBlock(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to add proposal", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	blockMtc.WithLabelValues("MintGas").Set(float64(blk.GasUsed()))
	blockMtc.WithLabelValues("MintActions").Set(float64(len(blk.Actions)))
	return blk, nil
}

// TipHeight returns tip block's height
func (cm *chainManager) TipHeight() uint64 {
	return cm.bc.TipHeight()
}

func (fc *forkChain) TipHeight() uint64 {
	return fc.head.Height()
}

// TipHash returns tip block's hash
func (fc *forkChain) TipHash() hash.Hash256 {
	return fc.tipHash()
}

func (cm *chainManager) StateReader() (protocol.StateReader, error) {
	return cm.srf.StateReaderAt(cm.bc.TipHeight(), cm.bc.TipHash())
}

// StateReader returns the state reader
func (fc *forkChain) StateReader() (protocol.StateReader, error) {
	return fc.sr, nil
}

// ValidateBlock validates a new block before adding it to the blockchain
func (cm *chainManager) ValidateBlock(blk *block.Block) error {
	err := cm.bc.ValidateBlock(blk)
	if err != nil {
		return err
	}
	if err = cm.pool.AddBlock(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to add proposal", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	return nil
}

// CommitBlock validates and appends a block to the chain
func (cm *chainManager) CommitBlock(blk *block.Block) error {
	if err := cm.bc.CommitBlock(blk); err != nil {
		return err
	}
	if err := cm.bbf.ReceiveBlock(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to receive block", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	if err := cm.pool.ReceiveBlock(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to receive block", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	return nil
}

func (cm *chainManager) Start(ctx context.Context) error {
	cm.pool.Init(cm.bc.TipHash())
	return nil
}

// Fork creates a new chain manager with the given hash
func (cm *chainManager) Fork(hash hash.Hash256) (ForkChain, error) {
	var (
		head *block.Header
		err  error
		tip  = cm.tipInfo()
	)
	if hash != tip.Hash {
		blk := cm.pool.BlockByHash(hash)
		if blk == nil {
			return nil, errors.Errorf("block %x not found when fork", hash)
		}
		head = &blk.Header
	} else {
		head, err = cm.bc.BlockHeaderByHeight(tip.Height)
		if head == nil {
			return nil, errors.Errorf("block %x not found when fork", hash)
		}
	}
	sr, err := cm.srf.StateReaderAt(head.Height(), hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create state reader at %d, hash %x", head.Height(), head.HashBlock())
	}
	return newForkChain(cm, head, sr), nil
}

func (cm *chainManager) draftBlockByHeight(height uint64, tipHash hash.Hash256) *block.Block {
	for blk := cm.pool.BlockByHash(tipHash); blk != nil && blk.Height() >= height; blk = cm.pool.BlockByHash(blk.PrevHash()) {
		if blk.Height() == height {
			return blk
		}
	}
	return nil
}

func (cm *chainManager) TipHash() hash.Hash256 {
	if cm.bc.TipHeight() == 0 {
		g := cm.bc.Genesis()
		return g.Hash()
	}
	return cm.bc.TipHash()
}

func (fc *forkChain) tipHash() hash.Hash256 {
	if fc.head.Height() == 0 {
		g := fc.cm.bc.Genesis()
		return g.Hash()
	}
	return fc.head.HashBlock()
}

func (cm *chainManager) tipInfo() *protocol.TipInfo {
	height := cm.bc.TipHeight()
	if height == 0 {
		g := cm.bc.Genesis()
		return &protocol.TipInfo{
			Height:    0,
			Hash:      g.Hash(),
			Timestamp: time.Unix(g.Timestamp, 0),
		}
	}
	head, err := cm.bc.BlockHeaderByHeight(height)
	if err != nil {
		log.L().Error("failed to get the head block", zap.Error(err))
		return nil
	}

	return &protocol.TipInfo{
		Height:        head.Height(),
		GasUsed:       head.GasUsed(),
		Hash:          head.HashBlock(),
		Timestamp:     head.Timestamp(),
		BaseFee:       head.BaseFee(),
		BlobGasUsed:   head.BlobGasUsed(),
		ExcessBlobGas: head.ExcessBlobGas(),
	}
}

func (fc *forkChain) tipInfo() *protocol.TipInfo {
	return &protocol.TipInfo{
		Height:        fc.head.Height(),
		GasUsed:       fc.head.GasUsed(),
		Hash:          fc.head.HashBlock(),
		Timestamp:     fc.head.Timestamp(),
		BaseFee:       fc.head.BaseFee(),
		BlobGasUsed:   fc.head.BlobGasUsed(),
		ExcessBlobGas: fc.head.ExcessBlobGas(),
	}
}

func (cm *chainManager) header(height uint64, tipHash hash.Hash256) (*block.Header, error) {
	header, err := cm.bc.BlockHeaderByHeight(height)
	switch errors.Cause(err) {
	case nil:
		return header, nil
	case db.ErrNotExist:
		if blk := cm.draftBlockByHeight(height, tipHash); blk != nil {
			return &blk.Header, nil
		}
	}
	return nil, err
}

func (cm *chainManager) mintContext(
	ctx context.Context,
	timestamp time.Time,
	producer address.Address,
	tip *protocol.TipInfo,
) context.Context {
	g := cm.bc.Genesis()
	// blockchain context
	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Tip:          *tip,
				ChainID:      cm.bc.ChainID(),
				EvmNetworkID: cm.bc.EvmNetworkID(),
				GetBlockHash: func(u uint64) (hash.Hash256, error) {
					header, err := cm.header(u, tip.Hash)
					if err != nil {
						return hash.ZeroHash256, err
					}
					return header.HashBlock(), nil
				},
				GetBlockTime: func(height uint64) (time.Time, error) {
					return cm.getBlockTime(height, tip.Hash)
				},
			},
		),
		g,
	)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	// block context
	height := tip.Height + 1
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    height,
			BlockTimeStamp: timestamp,
			Producer:       producer,
			GasLimit:       g.BlockGasLimitByHeight(height),
			BaseFee:        protocol.CalcBaseFee(g.Blockchain, tip),
			ExcessBlobGas:  protocol.CalcExcessBlobGas(tip.ExcessBlobGas, tip.BlobGasUsed),
		})
	return protocol.WithFeatureCtx(ctx)
}

func (cm *chainManager) getBlockTime(height uint64, tipHash hash.Hash256) (time.Time, error) {
	if height == 0 {
		return time.Unix(cm.bc.Genesis().Timestamp, 0), nil
	}
	header, err := cm.header(height, tipHash)
	if err != nil {
		return time.Time{}, err
	}
	return header.Timestamp(), nil
}
