package rolldpos

import (
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
)

var (
	// _proposalPoolMtc tracks proposal pool operations
	_proposalPoolMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iotex_consensus_proposalpool_operations_total",
		Help: "Total number of proposal pool operations",
	}, []string{"operation", "result"})

	// _proposalPoolSizeGauge tracks the current size of the proposal pool
	_proposalPoolSizeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iotex_consensus_proposalpool_size",
		Help: "Current number of blocks in the proposal pool",
	})

	// _proposalPoolForksGauge tracks the current number of forks
	_proposalPoolForksGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iotex_consensus_proposalpool_forks",
		Help: "Current number of forks in the proposal pool",
	})

	// _proposalPoolLatencyTimer tracks operation latency
	_proposalPoolLatencyTimer *prometheustimer.TimerFactory
)

func init() {
	prometheus.MustRegister(_proposalPoolMtc)
	prometheus.MustRegister(_proposalPoolSizeGauge)
	prometheus.MustRegister(_proposalPoolForksGauge)

	var err error
	_proposalPoolLatencyTimer, err = prometheustimer.New(
		"iotex_consensus_proposalpool_latency_nanoseconds",
		"Latency of proposal pool operations in nanoseconds",
		[]string{"operation"},
		[]string{"unknown"},
	)
	if err != nil {
		log.L().Error("Failed to create proposal pool latency timer", zap.Error(err))
	}
}

// proposalPool is a pool of draft blocks
type proposalPool struct {
	// nodes is a map of draft proposal blocks
	// key is the hash of the block
	nodes map[hash.Hash256]*block.Block
	// leaves is a map of tip blocks of forks
	// key is the hash of the tip block of the fork
	// value is the timestamp of the block
	leaves map[hash.Hash256]time.Time
	// root is the hash of the tip block of the blockchain
	root hash.Hash256
	mu   sync.Mutex
}

func newProposalPool() *proposalPool {
	pool := &proposalPool{
		nodes:  make(map[hash.Hash256]*block.Block),
		leaves: make(map[hash.Hash256]time.Time),
	}
	// Initialize metrics
	_proposalPoolSizeGauge.Set(0)
	_proposalPoolForksGauge.Set(0)
	return pool
}

func (d *proposalPool) Init(root hash.Hash256) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.root = root

	// Update metrics after initialization
	d.updateMetrics()

	log.L().Debug("proposal pool initialized", log.Hex("root", root[:]))
}

// AddBlock adds a block to the draft pool
func (d *proposalPool) AddBlock(blk *block.Block) error {
	timer := _proposalPoolLatencyTimer.NewTimer("add_block")
	defer timer.End()

	d.mu.Lock()
	defer d.mu.Unlock()

	// nothing to do if the block already exists
	hash := blk.HashBlock()
	if _, ok := d.nodes[hash]; ok {
		_proposalPoolMtc.WithLabelValues("add_block", "duplicate").Inc()
		return nil
	}
	// it must be a new tip of any fork, or make a new fork
	prevHash := blk.PrevHash()
	if _, ok := d.leaves[prevHash]; ok {
		delete(d.leaves, prevHash)
	} else if prevHash != d.root && d.nodes[prevHash] == nil {
		_proposalPoolMtc.WithLabelValues("add_block", "invalid_prev").Inc()
		return errors.Errorf("block %x is not a tip of any fork", prevHash[:])
	}
	d.leaves[hash] = blk.Timestamp()
	d.nodes[hash] = blk

	// Update metrics after successful addition
	d.updateMetrics()
	_proposalPoolMtc.WithLabelValues("add_block", "success").Inc()

	log.L().Debug("added block to draft pool", log.Hex("hash", hash[:]), zap.Uint64("height", blk.Height()), zap.Time("timestamp", blk.Timestamp()))
	return nil
}

// ReceiveBlock a block has been confirmed and remove all the blocks that is previous to it, or other forks
func (d *proposalPool) ReceiveBlock(blk *block.Block) error {
	timer := _proposalPoolLatencyTimer.NewTimer("receive_block")
	defer timer.End()

	d.mu.Lock()
	defer d.mu.Unlock()

	contain := func(node, leaf hash.Hash256) (exist bool) {
		for b := leaf; ; {
			if b == node {
				return true
			}
			blk := d.nodes[b]
			if blk == nil {
				return false
			}
			b = blk.PrevHash()
		}
	}

	// remove blocks in other forks or older blocks in the same fork
	leavesToDelete := make([]hash.Hash256, 0)
	for leaf := range d.leaves {
		start := d.nodes[leaf]
		has := contain(blk.HashBlock(), leaf)
		if has {
			start = blk
		}
		for b := start; b != nil; b = d.nodes[b.PrevHash()] {
			ha := b.HashBlock()
			log.L().Debug("remove block from draft pool", log.Hex("hash", ha[:]), zap.Uint64("height", b.Height()), zap.Time("timestamp", b.Timestamp()))
			delete(d.nodes, b.HashBlock())
		}
		if !has || blk.HashBlock() == leaf {
			leavesToDelete = append(leavesToDelete, leaf)
		}
	}
	// reset forks to only this one
	for _, f := range leavesToDelete {
		delete(d.leaves, f)
	}
	d.root = blk.HashBlock()

	// Update metrics after successful operation
	d.updateMetrics()
	_proposalPoolMtc.WithLabelValues("receive_block", "success").Inc()

	return nil
}

// BlockByHash returns the block by hash
func (d *proposalPool) BlockByHash(hash hash.Hash256) *block.Block {
	timer := _proposalPoolLatencyTimer.NewTimer("block_by_hash")
	defer timer.End()

	d.mu.Lock()
	defer d.mu.Unlock()

	block := d.nodes[hash]
	if block != nil {
		_proposalPoolMtc.WithLabelValues("block_by_hash", "hit").Inc()
	} else {
		_proposalPoolMtc.WithLabelValues("block_by_hash", "miss").Inc()
	}
	return block
}

// updateMetrics updates the gauge metrics with current pool state
func (d *proposalPool) updateMetrics() {
	_proposalPoolSizeGauge.Set(float64(len(d.nodes)))
	_proposalPoolForksGauge.Set(float64(len(d.leaves)))
}
