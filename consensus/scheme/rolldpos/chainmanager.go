package rolldpos

import (
	"context"
	"strconv"
	"time"

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
		Start(ctx context.Context) error
		ForkChain
		// Fork creates a new chain manager with the given hash
		Fork(hash hash.Hash256) (ChainManager, error)
	}
	// ForkChain defines the fork chain interface
	ForkChain interface {
		// BlockProposeTime return propose time by height
		BlockProposeTime(uint64) (time.Time, error)
		// BlockCommitTime return commit time by height
		BlockCommitTime(uint64) (time.Time, error)
		// TipHeight returns tip block's height
		TipHeight() uint64
		// TipHash returns tip block's hash
		TipHash() hash.Hash256
		// ChainAddress returns chain address on parent chain, the root chain return empty.
		ChainAddress() string
		// StateReader returns the state reader
		StateReader() protocol.StateReader
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(ctx context.Context) (*block.Block, error)
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(blk *block.Block) error
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
	}

	StateReaderFactory interface {
		StateReaderAt(*block.Header) (protocol.StateReader, error)
	}

	// BlockBuilderFactory is the factory interface of block builder
	BlockBuilderFactory interface {
		Mint(ctx context.Context) (*block.Block, error)
		ReceiveBlock(*block.Block) error
		Init(hash.Hash256)
		AddProposal(*block.Block) error
		Block(hash.Hash256) *block.Block
	}

	chainManager struct {
		*forkChain
		srf StateReaderFactory
	}

	forkChain struct {
		bc           blockchain.Blockchain
		head         *block.Header
		sr           protocol.StateReader
		timerFactory *prometheustimer.TimerFactory
		bbf          BlockBuilderFactory
	}
)

func init() {
	prometheus.MustRegister(blockMtc)
}

func newForkChain(bc blockchain.Blockchain, head *block.Header, sr protocol.StateReader, bbf BlockBuilderFactory) *forkChain {
	timerFactory, err := prometheustimer.New(
		"iotex_blockchain_perf",
		"Performance of blockchain module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(bc.ChainID()), 10)},
	)
	if err != nil {
		log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	return &forkChain{
		bc:           bc,
		head:         head,
		sr:           sr,
		bbf:          bbf,
		timerFactory: timerFactory,
	}
}

// NewChainManager creates a chain manager
func NewChainManager(bc blockchain.Blockchain, srf StateReaderFactory, bbf BlockBuilderFactory) ChainManager {
	return &chainManager{
		forkChain: newForkChain(bc, nil, nil, bbf),
		srf:       srf,
	}
}

// BlockProposeTime return propose time by height
func (cm *forkChain) BlockProposeTime(height uint64) (time.Time, error) {
	if height == 0 {
		return time.Unix(cm.bc.Genesis().Timestamp, 0), nil
	}
	header, err := cm.bc.BlockHeaderByHeight(height)
	switch errors.Cause(err) {
	case nil:
		return header.Timestamp(), nil
	case db.ErrNotExist:
		if blk := cm.draftBlockByHeight(height); blk != nil {
			return blk.Timestamp(), nil
		}
		return time.Time{}, err
	default:
		return time.Time{}, err
	}
}

// BlockCommitTime return commit time by height
func (cm *forkChain) BlockCommitTime(height uint64) (time.Time, error) {
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
func (fc *forkChain) MintNewBlock(ctx context.Context) (*block.Block, error) {
	mintNewBlockTimer := fc.timerFactory.NewTimer("MintBlock")
	defer mintNewBlockTimer.End()
	var (
		roundCtx       = protocol.MustGetConsensusRoundCtx(ctx)
		tipHeight      = fc.head.Height()
		tipHash        = fc.tipHash()
		newblockHeight = tipHeight + 1
		timestamp      = roundCtx.StartTime
		producer       address.Address
		err            error
	)
	// safety check
	if roundCtx.Height != tipHeight {
		return nil, errors.Errorf("invalid height, expecting %d, got %d", tipHeight, roundCtx.Height)
	}
	if roundCtx.PrevHash != tipHash {
		return nil, errors.Errorf("invalid prev hash, expecting %x, got %x", tipHash, roundCtx.PrevHash)
	}
	producer, err = address.FromString(roundCtx.EncodedProposer)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse producer address %s", roundCtx.EncodedProposer)
	}
	ctx = fc.mintContext(context.Background(), roundCtx.StartTime, producer)
	// create a new block
	log.L().Debug("Produce a new block.", zap.Uint64("height", newblockHeight), zap.Time("timestamp", timestamp), log.Hex("prevHash", tipHash[:]))
	// run execution and update state trie root hash
	blk, err := fc.bbf.Mint(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}
	if err = fc.bbf.AddProposal(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to add proposal", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	blockMtc.WithLabelValues("MintGas").Set(float64(blk.GasUsed()))
	blockMtc.WithLabelValues("MintActions").Set(float64(len(blk.Actions)))
	return blk, nil
}

// TipHeight returns tip block's height
func (cm *forkChain) TipHeight() uint64 {
	return cm.head.Height()
}

// TipHash returns tip block's hash
func (cm *forkChain) TipHash() hash.Hash256 {
	return cm.tipHash()
}

// ChainAddress returns chain address on parent chain, the root chain return empty.
func (cm *forkChain) ChainAddress() string {
	return cm.bc.ChainAddress()
}

// StateReader returns the state reader
func (cm *forkChain) StateReader() protocol.StateReader {
	return cm.sr
}

// ValidateBlock validates a new block before adding it to the blockchain
func (cm *forkChain) ValidateBlock(blk *block.Block) error {
	err := cm.bc.ValidateBlock(blk)
	if err != nil {
		return err
	}
	if err = cm.bbf.AddProposal(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to add proposal", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	return nil
}

// CommitBlock validates and appends a block to the chain
func (cm *forkChain) CommitBlock(blk *block.Block) error {
	if err := cm.bc.CommitBlock(blk); err != nil {
		return err
	}
	if err := cm.bbf.ReceiveBlock(blk); err != nil {
		blkHash := blk.HashBlock()
		log.L().Error("failed to receive block", zap.Error(err), zap.Uint64("height", blk.Height()), log.Hex("hash", blkHash[:]))
	}
	return nil
}

func (cm *chainManager) Start(ctx context.Context) error {
	head, err := cm.bc.BlockHeaderByHeight(cm.bc.TipHeight())
	if err != nil {
		return errors.Wrap(err, "failed to get the head block")
	}
	sr, err := cm.srf.StateReaderAt(head)
	if err != nil {
		return errors.Wrapf(err, "failed to create state reader at %d, hash %x", head.Height(), head.HashBlock())
	}
	cm.forkChain = newForkChain(cm.bc, head, sr, cm.bbf)
	cm.bbf.Init(cm.forkChain.TipHash())
	return nil
}

// Fork creates a new chain manager with the given hash
func (cm *chainManager) Fork(hash hash.Hash256) (ChainManager, error) {
	head := cm.head
	if hash != cm.tipHash() {
		blk := cm.bbf.Block(hash)
		if blk == nil {
			return nil, errors.Errorf("block %x not found when fork", hash)
		}
		head = &blk.Header
	}
	sr, err := cm.srf.StateReaderAt(head)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create state reader at %d, hash %x", head.Height(), head.HashBlock())
	}
	return &chainManager{
		srf:       cm.srf,
		forkChain: newForkChain(cm.bc, head, sr, cm.bbf),
	}, nil
}

func (fc *forkChain) draftBlockByHeight(height uint64) *block.Block {
	for blk := fc.bbf.Block(fc.tipHash()); blk != nil && blk.Height() >= height; blk = fc.bbf.Block(blk.PrevHash()) {
		if blk.Height() == height {
			return blk
		}
	}
	return nil
}

func (fc *forkChain) tipHash() hash.Hash256 {
	if fc.head.Height() == 0 {
		g := fc.bc.Genesis()
		return g.Hash()
	}
	return fc.head.HashBlock()
}

func (fc *forkChain) tipInfo() *protocol.TipInfo {
	if fc.head.Height() == 0 {
		g := fc.bc.Genesis()
		return &protocol.TipInfo{
			Height:    0,
			Hash:      g.Hash(),
			Timestamp: time.Unix(g.Timestamp, 0),
		}
	}
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

func (fc *forkChain) mintContext(ctx context.Context,
	timestamp time.Time, producer address.Address) context.Context {
	// blockchain context
	tip := fc.tipInfo()
	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Tip:          *tip,
				ChainID:      fc.bc.ChainID(),
				EvmNetworkID: fc.bc.EvmNetworkID(),
			},
		),
		fc.bc.Genesis(),
	)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	// block context
	g := fc.bc.Genesis()
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
	ctx = protocol.WithFeatureCtx(ctx)
	return ctx
}
