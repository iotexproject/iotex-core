// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package blobpool implements the EIP-4844 blob transaction pool.
package blobpool

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/billy"
	"github.com/holiman/uint256"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

const (
	// blobSize is the protocol constrained byte size of a single blob in a
	// transaction. There can be multiple of these embedded into a single tx.
	blobSize = params.BlobTxFieldElementsPerBlob * params.BlobTxBytesPerFieldElement

	// maxBlobsPerTransaction is the maximum number of blobs a single transaction
	// is allowed to contain. Whilst the spec states it's unlimited, the block
	// data slots are protocol bound, which implicitly also limit this.
	maxBlobsPerTransaction = params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob

	// txAvgSize is an approximate byte size of a transaction metadata to avoid
	// tiny overflows causing all txs to move a shelf higher, wasting disk space.
	txAvgSize = 4 * 1024

	// txMaxSize is the maximum size a single transaction can have, outside
	// the included blobs. Since blob transactions are pulled instead of pushed,
	// and only a small metadata is kept in ram, the rest is on disk, there is
	// no critical limit that should be enforced. Still, capping it to some sane
	// limit can never hurt.
	txMaxSize = 1024 * 1024

	// maxTxsPerAccount is the maximum number of blob transactions admitted from
	// a single account. The limit is enforced to minimize the DoS potential of
	// a private tx cancelling publicly propagated blobs.
	//
	// Note, transactions resurrected by a reorg are also subject to this limit,
	// so pushing it down too aggressively might make resurrections non-functional.
	maxTxsPerAccount = 16

	// pendingTransactionStore is the subfolder containing the currently queued
	// blob transactions.
	pendingTransactionStore = "queue"

	// limboedTransactionStore is the subfolder containing the currently included
	// but not yet finalized transaction blobs.
	limboedTransactionStore = "limbo"
)

// blobTxMeta is the minimal subset of types.BlobTx necessary to validate and
// schedule the blob transactions into the following blocks. Only ever add the
// bare minimum needed fields to keep the size down (and thus number of entries
// larger with the same memory consumption).
type blobTxMeta struct {
	hash common.Hash // Transaction hash to maintain the lookup table
	id   uint64      // Storage ID in the pool's persistent store
	size uint32      // Byte size in the pool's persistent store

	nonce      uint64       // Needed to prioritize inclusion order within an account
	costCap    *uint256.Int // Needed to validate cumulative balance sufficiency
	execTipCap *uint256.Int // Needed to prioritize inclusion order across accounts and validate replacement price bump
	execFeeCap *uint256.Int // Needed to validate replacement price bump
	blobFeeCap *uint256.Int // Needed to validate replacement price bump
	execGas    uint64       // Needed to check inclusion validity before reading the blob
	blobGas    uint64       // Needed to check inclusion validity before reading the blob

	basefeeJumps float64 // Absolute number of 1559 fee adjustments needed to reach the tx's fee cap
	blobfeeJumps float64 // Absolute number of 4844 fee adjustments needed to reach the tx's blob fee cap

	evictionExecTip      *uint256.Int // Worst gas tip across all previous nonces
	evictionExecFeeJumps float64      // Worst base fee (converted to fee jumps) across all previous nonces
	evictionBlobFeeJumps float64      // Worse blob fee (converted to fee jumps) across all previous nonces
}

// newBlobTxMeta retrieves the indexed metadata fields from a blob transaction
// and assembles a helper struct to track in memory.
func newBlobTxMeta(id uint64, size uint32, tx *action.SealedEnvelope) *blobTxMeta {
	h, _ := tx.Hash()
	cost, _ := tx.Cost()
	meta := &blobTxMeta{
		hash:       common.BytesToHash(h[:]),
		id:         id,
		size:       size,
		nonce:      tx.Nonce(),
		costCap:    uint256.MustFromBig(cost),
		execTipCap: uint256.MustFromBig(tx.GasTipCap()),
		execFeeCap: uint256.MustFromBig(tx.GasFeeCap()),
		blobFeeCap: uint256.MustFromBig(BlobGasFeeCap(tx)),
		execGas:    tx.GasLimit(),
		blobGas:    BlobGasLimit(tx),
	}
	meta.basefeeJumps = dynamicFeeJumps(meta.execFeeCap)
	meta.blobfeeJumps = dynamicFeeJumps(meta.blobFeeCap)

	return meta
}

// BlobPool is the transaction pool dedicated to EIP-4844 blob transactions.
//
// Blob transactions are special snowflakes that are designed for a very specific
// purpose (rollups) and are expected to adhere to that specific use case. These
// behavioural expectations allow us to design a transaction pool that is more robust
// (i.e. resending issues) and more resilient to DoS attacks (e.g. replace-flush
// attacks) than the generic tx pool. These improvements will also mean, however,
// that we enforce a significantly more aggressive strategy on entering and exiting
// the pool:
//
//   - Blob transactions are large. With the initial design aiming for 128KB blobs,
//     we must ensure that these only traverse the network the absolute minimum
//     number of times. Broadcasting to sqrt(peers) is out of the question, rather
//     these should only ever be announced and the remote side should request it if
//     it wants to.
//
//   - Block blob-space is limited. With blocks being capped to a few blob txs, we
//     can make use of the very low expected churn rate within the pool. Notably,
//     we should be able to use a persistent disk backend for the pool, solving
//     the tx resend issue that plagues the generic tx pool, as long as there's no
//     artificial churn (i.e. pool wars).
//
//   - Purpose of blobs are layer-2s. Layer-2s are meant to use blob transactions to
//     commit to their own current state, which is independent of Ethereum mainnet
//     (state, txs). This means that there's no reason for blob tx cancellation or
//     replacement, apart from a potential basefee / miner tip adjustment.
//
//   - Replacements are expensive. Given their size, propagating a replacement
//     blob transaction to an existing one should be aggressively discouraged.
//     Whilst generic transactions can start at 1 Wei gas cost and require a 10%
//     fee bump to replace, we suggest requiring a higher min cost (e.g. 1 gwei)
//     and a more aggressive bump (100%).
//
//   - Cancellation is prohibitive. Evicting an already propagated blob tx is a huge
//     DoS vector. As such, a) replacement (higher-fee) blob txs mustn't invalidate
//     already propagated (future) blob txs (cumulative fee); b) nonce-gapped blob
//     txs are disallowed; c) the presence of blob transactions exclude non-blob
//     transactions.
//
//   - Malicious cancellations are possible. Although the pool might prevent txs
//     that cancel blobs, blocks might contain such transaction (malicious miner
//     or flashbotter). The pool should cap the total number of blob transactions
//     per account as to prevent propagating too much data before cancelling it
//     via a normal transaction. It should nonetheless be high enough to support
//     resurrecting reorged transactions. Perhaps 4-16.
//
//   - Local txs are meaningless. Mining pools historically used local transactions
//     for payouts or for backdoor deals. With 1559 in place, the basefee usually
//     dominates the final price, so 0 or non-0 tip doesn't change much. Blob txs
//     retain the 1559 2D gas pricing (and introduce on top a dynamic blob gas fee),
//     so locality is moot. With a disk backed blob pool avoiding the resend issue,
//     there's also no need to save own transactions for later.
//
//   - No-blob blob-txs are bad. Theoretically there's no strong reason to disallow
//     blob txs containing 0 blobs. In practice, admitting such txs into the pool
//     breaks the low-churn invariant as blob constraints don't apply anymore. Even
//     though we could accept blocks containing such txs, a reorg would require moving
//     them back into the blob pool, which can break invariants.
//
//   - Dropping blobs needs delay. When normal transactions are included, they
//     are immediately evicted from the pool since they are contained in the
//     including block. Blobs however are not included in the execution chain,
//     so a mini reorg cannot re-pool "lost" blob transactions. To support reorgs,
//     blobs are retained on disk until they are finalised.
//
//   - Blobs can arrive via flashbots. Blocks might contain blob transactions we
//     have never seen on the network. Since we cannot recover them from blocks
//     either, the engine_newPayload needs to give them to us, and we cache them
//     until finality to support reorgs without tx losses.
//
// Whilst some constraints above might sound overly aggressive, the general idea is
// that the blob pool should work robustly for its intended use case and whilst
// anyone is free to use blob transactions for arbitrary non-rollup use cases,
// they should not be allowed to run amok the network.
//
// Implementation wise there are a few interesting design choices:
//
//   - Adding a transaction to the pool blocks until persisted to disk. This is
//     viable because TPS is low (2-4 blobs per block initially, maybe 8-16 at
//     peak), so natural churn is a couple MB per block. Replacements doing O(n)
//     updates are forbidden and transaction propagation is pull based (i.e. no
//     pileup of pending data).
//
//   - When transactions are chosen for inclusion, the primary criteria is the
//     signer tip (and having a basefee/data fee high enough of course). However,
//     same-tip transactions will be split by their basefee/datafee, preferring
//     those that are closer to the current network limits. The idea being that
//     very relaxed ones can be included even if the fees go up, when the closer
//     ones could already be invalid.
//
// When the pool eventually reaches saturation, some old transactions - that may
// never execute - will need to be evicted in favor of newer ones. The eviction
// strategy is quite complex:
//
//   - Exceeding capacity evicts the highest-nonce of the account with the lowest
//     paying blob transaction anywhere in the pooled nonce-sequence, as that tx
//     would be executed the furthest in the future and is thus blocking anything
//     after it. The smallest is deliberately not evicted to avoid a nonce-gap.
//
//   - Analogously, if the pool is full, the consideration price of a new tx for
//     evicting an old one is the smallest price in the entire nonce-sequence of
//     the account. This avoids malicious users DoSing the pool with seemingly
//     high paying transactions hidden behind a low-paying blocked one.
//
//   - Since blob transactions have 3 price parameters: execution tip, execution
//     fee cap and data fee cap, there's no singular parameter to create a total
//     price ordering on. What's more, since the base fee and blob fee can move
//     independently of one another, there's no pre-defined way to combine them
//     into a stable order either. This leads to a multi-dimensional problem to
//     solve after every block.
//
//   - The first observation is that comparing 1559 base fees or 4844 blob fees
//     needs to happen in the context of their dynamism. Since these fees jump
//     up or down in ~1.125 multipliers (at max) across blocks, comparing fees
//     in two transactions should be based on log1.125(fee) to eliminate noise.
//
//   - The second observation is that the basefee and blobfee move independently,
//     so there's no way to split mixed txs on their own (A has higher base fee,
//     B has higher blob fee). Rather than look at the absolute fees, the useful
//     metric is the max time it can take to exceed the transaction's fee caps.
//     Specifically, we're interested in the number of jumps needed to go from
//     the current fee to the transaction's cap:
//
//     jumps = log1.125(txfee) - log1.125(basefee)
//
//   - The third observation is that the base fee tends to hover around rather
//     than swing wildly. The number of jumps needed from the current fee starts
//     to get less relevant the higher it is. To remove the noise here too, the
//     pool will use log(jumps) as the delta for comparing transactions.
//
//     delta = sign(jumps) * log(abs(jumps))
//
//   - To establish a total order, we need to reduce the dimensionality of the
//     two base fees (log jumps) to a single value. The interesting aspect from
//     the pool's perspective is how fast will a tx get executable (fees going
//     down, crossing the smaller negative jump counter) or non-executable (fees
//     going up, crossing the smaller positive jump counter). As such, the pool
//     cares only about the min of the two delta values for eviction priority.
//
//     priority = min(delta-basefee, delta-blobfee)
//
//   - The above very aggressive dimensionality and noise reduction should result
//     in transaction being grouped into a small number of buckets, the further
//     the fees the larger the buckets. This is good because it allows us to use
//     the miner tip meaningfully as a splitter.
//
//   - For the scenario where the pool does not contain non-executable blob txs
//     anymore, it does not make sense to grant a later eviction priority to txs
//     with high fee caps since it could enable pool wars. As such, any positive
//     priority will be grouped together.
//
//     priority = min(delta-basefee, delta-blobfee, 0)
//
// Optimisation tradeoffs:
//
//   - Eviction relies on 3 fee minimums per account (exec tip, exec cap and blob
//     cap). Maintaining these values across all transactions from the account is
//     problematic as each transaction replacement or inclusion would require a
//     rescan of all other transactions to recalculate the minimum. Instead, the
//     pool maintains a rolling minimum across the nonce range. Updating all the
//     minimums will need to be done only starting at the swapped in/out nonce
//     and leading up to the first no-change.
type BlobPool struct {
	config  Config                 // Pool configuration
	reserve txpool.AddressReserver // Address reserver to ensure exclusivity across subpools

	store  billy.Database // Persistent data store for the tx metadata and blobs
	stored uint64         // Useful data size of all transactions on disk

	signer types.Signer // Transaction signer to use for sender recovery
	chain  BlockChain   // Chain object to access the state through

	head   *block.Header // Current head of the chain
	state  *stateAdapter // Current state at the head of the chain
	gasTip *uint256.Int  // Currently accepted minimum gas tip

	lookup map[common.Hash]uint64           // Lookup table mapping hashes to tx billy entries
	index  map[common.Address][]*blobTxMeta // Blob transactions grouped by accounts, sorted by nonce
	spent  map[common.Address]*uint256.Int  // Expenditure tracking for individual accounts
	evict  *evictHeap                       // Heap of cheapest accounts for eviction when full

	lock sync.RWMutex // Mutex protecting the pool during reorg handling
}

// New creates a new blob transaction pool to gather, sort and filter inbound
// blob transactions from the network.
func New(config Config, signer types.Signer, chain BlockChain) *BlobPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the transaction pool with its initial settings
	return &BlobPool{
		config: config,
		signer: signer,
		chain:  chain,
		lookup: make(map[common.Hash]uint64),
		index:  make(map[common.Address][]*blobTxMeta),
		spent:  make(map[common.Address]*uint256.Int),
	}
}

// Filter returns whether the given transaction can be consumed by the blob pool.
func (p *BlobPool) Filter(tx *action.SealedEnvelope) bool {
	return TxType(tx) == types.BlobTxType
}

// Init sets the gas price needed to keep a transaction in the pool and the chain
// head to allow balance / nonce checks. The transaction journal will be loaded
// from disk and filtered based on the provided starting settings.
func (p *BlobPool) Init(gasTip *big.Int, head *block.Header, sf protocol.StateReader, reserve txpool.AddressReserver) error {
	p.reserve = reserve

	var (
		queuedir string
		limbodir string
	)
	if p.config.Datadir != "" {
		queuedir = filepath.Join(p.config.Datadir, pendingTransactionStore)
		if err := os.MkdirAll(queuedir, 0700); err != nil {
			return err
		}
		limbodir = filepath.Join(p.config.Datadir, limboedTransactionStore)
		if err := os.MkdirAll(limbodir, 0700); err != nil {
			return err
		}
	}
	// Initialize the state with head block, or fallback to empty one in
	// case the head state is not available(might occur when node is not
	// fully synced).
	p.head, p.state = head, newStateAdapter(p.chain.Config(), sf)

	// Index all transactions on disk and delete anything inprocessable
	var fails []uint64
	index := func(id uint64, size uint32, blob []byte) {
		if p.parseTransaction(id, size, blob) != nil {
			fails = append(fails, id)
		}
	}
	store, err := billy.Open(billy.Options{Path: queuedir}, newSlotter(), index)
	if err != nil {
		return err
	}
	p.store = store

	if len(fails) > 0 {
		log.Warn("Dropping invalidated blob transactions", "ids", fails)
		dropInvalidMeter.Mark(int64(len(fails)))

		for _, id := range fails {
			if err := p.store.Delete(id); err != nil {
				p.Close()
				return err
			}
		}
	}
	// Sort the indexed transactions by nonce and delete anything gapped, create
	// the eviction heap of anyone still standing
	for addr := range p.index {
		p.recheck(addr, nil)
	}
	var (
		basefee = uint256.MustFromBig(eip1559CalcBaseFee())
		blobfee = uint256.MustFromBig(big.NewInt(params.BlobTxMinBlobGasprice))
	)
	if ExcessBlobGas(p.head) != nil {
		blobfee = uint256.MustFromBig(eip4844CalcBlobFee())
	}
	p.evict = newPriceHeap(basefee, blobfee, &p.index)

	// Set the configured gas tip, triggering a filtering of anything just loaded
	basefeeGauge.Update(int64(basefee.Uint64()))
	blobfeeGauge.Update(int64(blobfee.Uint64()))

	p.SetGasTip(gasTip)

	// Since the user might have modified their pool's capacity, evict anything
	// above the current allowance
	for p.stored > p.config.Datacap {
		p.drop()
	}
	// Update the metrics and return the constructed pool
	datacapGauge.Update(int64(p.config.Datacap))
	p.updateStorageMetrics()
	return nil
}

// Close closes down the underlying persistent store.
func (p *BlobPool) Close() error {
	var errs []error
	if err := p.store.Close(); err != nil {
		errs = append(errs, err)
	}
	switch {
	case errs == nil:
		return nil
	case len(errs) == 1:
		return errs[0]
	default:
		return fmt.Errorf("%v", errs)
	}
}

// parseTransaction is a callback method on pool creation that gets called for
// each transaction on disk to create the in-memory metadata index.
func (p *BlobPool) parseTransaction(id uint64, size uint32, blob []byte) error {
	tx := new(action.SealedEnvelope)
	if err := DecodeAction(blob, tx); err != nil {
		// This path is impossible unless the disk data representation changes
		// across restarts. For that ever improbable case, recover gracefully
		// by ignoring this data entry.
		log.Error("Failed to decode blob pool entry", "id", id, "err", err)
		return err
	}
	if BlobTxSidecar(tx) == nil {
		h, _ := tx.Hash()
		log.Error("Missing sidecar in blob pool entry", "id", id, "hash", common.BytesToHash(h[:]))
		return errors.New("missing blob sidecar")
	}

	meta := newBlobTxMeta(id, size, tx)
	if _, exists := p.lookup[meta.hash]; exists {
		// This path is only possible after a crash, where deleted items are not
		// removed via the normal shutdown-startup procedure and thus may get
		// partially resurrected.
		h, _ := tx.Hash()
		log.Error("Rejecting duplicate blob pool entry", "id", id, "hash", common.BytesToHash(h[:]))
		return errors.New("duplicate blob entry")
	}
	sender := common.BytesToAddress(tx.SenderAddress().Bytes())
	if _, ok := p.index[sender]; !ok {
		if err := p.reserve(sender, true); err != nil {
			return err
		}
		p.index[sender] = []*blobTxMeta{}
		p.spent[sender] = new(uint256.Int)
	}
	p.index[sender] = append(p.index[sender], meta)
	p.spent[sender] = new(uint256.Int).Add(p.spent[sender], meta.costCap)

	p.lookup[meta.hash] = meta.id
	p.stored += uint64(meta.size)

	return nil
}

// recheck verifies the pool's content for a specific account and drops anything
// that does not fit anymore (dangling or filled nonce, overdraft).
func (p *BlobPool) recheck(addr common.Address, inclusions map[common.Hash]uint64) {
	// Sort the transactions belonging to the account so reinjects can be simpler
	txs := p.index[addr]
	if inclusions != nil && txs == nil { // during reorgs, we might find new accounts
		return
	}
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].nonce < txs[j].nonce
	})
	// If there is a gap between the chain state and the blob pool, drop
	// all the transactions as they are non-executable. Similarly, if the
	// entire tx range was included, drop all.
	var (
		next   = p.state.GetNonce(addr)
		gapped = txs[0].nonce > next
		filled = txs[len(txs)-1].nonce < next
	)
	if gapped || filled {
		var (
			ids    []uint64
			nonces []uint64
		)
		for i := 0; i < len(txs); i++ {
			ids = append(ids, txs[i].id)
			nonces = append(nonces, txs[i].nonce)

			p.stored -= uint64(txs[i].size)
			delete(p.lookup, txs[i].hash)
		}
		delete(p.index, addr)
		delete(p.spent, addr)
		if inclusions != nil { // only during reorgs will the heap will be initialized
			heap.Remove(p.evict, p.evict.index[addr])
		}
		p.reserve(addr, false)

		if gapped {
			log.Warn("Dropping dangling blob transactions", "from", addr, "missing", next, "drop", nonces, "ids", ids)
			dropDanglingMeter.Mark(int64(len(ids)))
		} else {
			log.Trace("Dropping filled blob transactions", "from", addr, "filled", nonces, "ids", ids)
			dropFilledMeter.Mark(int64(len(ids)))
		}
		for _, id := range ids {
			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
		}
		return
	}
	// If there is overlap between the chain state and the blob pool, drop
	// anything below the current state
	if txs[0].nonce < next {
		var (
			ids    []uint64
			nonces []uint64
		)
		for txs[0].nonce < next {
			ids = append(ids, txs[0].id)
			nonces = append(nonces, txs[0].nonce)

			p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], txs[0].costCap)
			p.stored -= uint64(txs[0].size)
			delete(p.lookup, txs[0].hash)

			txs = txs[1:]
		}
		log.Trace("Dropping overlapped blob transactions", "from", addr, "overlapped", nonces, "ids", ids, "left", len(txs))
		dropOverlappedMeter.Mark(int64(len(ids)))

		for _, id := range ids {
			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
		}
		p.index[addr] = txs
	}
	// Iterate over the transactions to initialize their eviction thresholds
	// and to detect any nonce gaps
	txs[0].evictionExecTip = txs[0].execTipCap
	txs[0].evictionExecFeeJumps = txs[0].basefeeJumps
	txs[0].evictionBlobFeeJumps = txs[0].blobfeeJumps

	for i := 1; i < len(txs); i++ {
		// If there's no nonce gap, initialize the eviction thresholds as the
		// minimum between the cumulative thresholds and the current tx fees
		if txs[i].nonce == txs[i-1].nonce+1 {
			txs[i].evictionExecTip = txs[i-1].evictionExecTip
			if txs[i].evictionExecTip.Cmp(txs[i].execTipCap) > 0 {
				txs[i].evictionExecTip = txs[i].execTipCap
			}
			txs[i].evictionExecFeeJumps = txs[i-1].evictionExecFeeJumps
			if txs[i].evictionExecFeeJumps > txs[i].basefeeJumps {
				txs[i].evictionExecFeeJumps = txs[i].basefeeJumps
			}
			txs[i].evictionBlobFeeJumps = txs[i-1].evictionBlobFeeJumps
			if txs[i].evictionBlobFeeJumps > txs[i].blobfeeJumps {
				txs[i].evictionBlobFeeJumps = txs[i].blobfeeJumps
			}
			continue
		}
		// Sanity check that there's no double nonce. This case would generally
		// be a coding error, so better know about it.
		//
		// Also, Billy behind the blobpool does not journal deletes. A process
		// crash would result in previously deleted entities being resurrected.
		// That could potentially cause a duplicate nonce to appear.
		if txs[i].nonce == txs[i-1].nonce {
			id := p.lookup[txs[i].hash]

			log.Error("Dropping repeat nonce blob transaction", "from", addr, "nonce", txs[i].nonce, "id", id)
			dropRepeatedMeter.Mark(1)

			p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], txs[i].costCap)
			p.stored -= uint64(txs[i].size)
			delete(p.lookup, txs[i].hash)

			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
			txs = append(txs[:i], txs[i+1:]...)
			p.index[addr] = txs

			i--
			continue
		}
		// Otherwise if there's a nonce gap evict all later transactions
		var (
			ids    []uint64
			nonces []uint64
		)
		for j := i; j < len(txs); j++ {
			ids = append(ids, txs[j].id)
			nonces = append(nonces, txs[j].nonce)

			p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], txs[j].costCap)
			p.stored -= uint64(txs[j].size)
			delete(p.lookup, txs[j].hash)
		}
		txs = txs[:i]

		log.Error("Dropping gapped blob transactions", "from", addr, "missing", txs[i-1].nonce+1, "drop", nonces, "ids", ids)
		dropGappedMeter.Mark(int64(len(ids)))

		for _, id := range ids {
			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
		}
		p.index[addr] = txs
		break
	}
	// Ensure that there's no over-draft, this is expected to happen when some
	// transactions get included without publishing on the network
	var (
		balance = p.state.GetBalance(addr)
		spent   = p.spent[addr]
	)
	if spent.Cmp(balance) > 0 {
		// Evict the highest nonce transactions until the pending set falls under
		// the account's available balance
		var (
			ids    []uint64
			nonces []uint64
		)
		for p.spent[addr].Cmp(balance) > 0 {
			last := txs[len(txs)-1]
			txs[len(txs)-1] = nil
			txs = txs[:len(txs)-1]

			ids = append(ids, last.id)
			nonces = append(nonces, last.nonce)

			p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], last.costCap)
			p.stored -= uint64(last.size)
			delete(p.lookup, last.hash)
		}
		if len(txs) == 0 {
			delete(p.index, addr)
			delete(p.spent, addr)
			if inclusions != nil { // only during reorgs will the heap will be initialized
				heap.Remove(p.evict, p.evict.index[addr])
			}
			p.reserve(addr, false)
		} else {
			p.index[addr] = txs
		}
		log.Warn("Dropping overdrafted blob transactions", "from", addr, "balance", balance, "spent", spent, "drop", nonces, "ids", ids)
		dropOverdraftedMeter.Mark(int64(len(ids)))

		for _, id := range ids {
			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
		}
	}
	// Sanity check that no account can have more queued transactions than the
	// DoS protection threshold.
	if len(txs) > maxTxsPerAccount {
		// Evict the highest nonce transactions until the pending set falls under
		// the account's transaction cap
		var (
			ids    []uint64
			nonces []uint64
		)
		for len(txs) > maxTxsPerAccount {
			last := txs[len(txs)-1]
			txs[len(txs)-1] = nil
			txs = txs[:len(txs)-1]

			ids = append(ids, last.id)
			nonces = append(nonces, last.nonce)

			p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], last.costCap)
			p.stored -= uint64(last.size)
			delete(p.lookup, last.hash)
		}
		p.index[addr] = txs

		log.Warn("Dropping overcapped blob transactions", "from", addr, "kept", len(txs), "drop", nonces, "ids", ids)
		dropOvercappedMeter.Mark(int64(len(ids)))

		for _, id := range ids {
			if err := p.store.Delete(id); err != nil {
				log.Error("Failed to delete blob transaction", "from", addr, "id", id, "err", err)
			}
		}
	}
	// Included cheap transactions might have left the remaining ones better from
	// an eviction point, fix any potential issues in the heap.
	if _, ok := p.index[addr]; ok && inclusions != nil {
		heap.Fix(p.evict, p.evict.index[addr])
	}
}

// Reset implements txpool.SubPool, allowing the blob pool's internal state to be
// kept in sync with the main transaction pool's internal state.
func (p *BlobPool) Reset(oldHead, newHead *block.Header) {
	waitStart := time.Now()
	p.lock.Lock()
	resetwaitHist.Update(time.Since(waitStart).Nanoseconds())
	defer p.lock.Unlock()

	defer func(start time.Time) {
		resettimeHist.Update(time.Since(start).Nanoseconds())
	}(time.Now())

	p.head = newHead

	// Reset the price heap for the new set of basefee/blobfee pairs
	var (
		basefee = uint256.MustFromBig(eip1559CalcBaseFee())
		blobfee = uint256.MustFromBig(big.NewInt(params.BlobTxMinBlobGasprice))
	)
	if ExcessBlobGas(newHead) != nil {
		blobfee = uint256.MustFromBig(eip4844.CalcBlobFee(*ExcessBlobGas(newHead)))
	}
	p.evict.reinit(basefee, blobfee, false)

	basefeeGauge.Update(int64(basefee.Uint64()))
	blobfeeGauge.Update(int64(blobfee.Uint64()))
	p.updateStorageMetrics()
}

// SetGasTip implements txpool.SubPool, allowing the blob pool's gas requirements
// to be kept in sync with the main transaction pool's gas requirements.
func (p *BlobPool) SetGasTip(tip *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Store the new minimum gas tip
	old := p.gasTip
	p.gasTip = uint256.MustFromBig(tip)

	// If the min miner fee increased, remove transactions below the new threshold
	if old == nil || p.gasTip.Cmp(old) > 0 {
		for addr, txs := range p.index {
			for i, tx := range txs {
				if tx.execTipCap.Cmp(p.gasTip) < 0 {
					// Drop the offending transaction
					var (
						ids    = []uint64{tx.id}
						nonces = []uint64{tx.nonce}
					)
					p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], txs[i].costCap)
					p.stored -= uint64(tx.size)
					delete(p.lookup, tx.hash)
					txs[i] = nil

					// Drop everything afterwards, no gaps allowed
					for j, tx := range txs[i+1:] {
						ids = append(ids, tx.id)
						nonces = append(nonces, tx.nonce)

						p.spent[addr] = new(uint256.Int).Sub(p.spent[addr], tx.costCap)
						p.stored -= uint64(tx.size)
						delete(p.lookup, tx.hash)
						txs[i+1+j] = nil
					}
					// Clear out the dropped transactions from the index
					if i > 0 {
						p.index[addr] = txs[:i]
						heap.Fix(p.evict, p.evict.index[addr])
					} else {
						delete(p.index, addr)
						delete(p.spent, addr)

						heap.Remove(p.evict, p.evict.index[addr])
						p.reserve(addr, false)
					}
					// Clear out the transactions from the data store
					log.Warn("Dropping underpriced blob transaction", "from", addr, "rejected", tx.nonce, "tip", tx.execTipCap, "want", tip, "drop", nonces, "ids", ids)
					dropUnderpricedMeter.Mark(int64(len(ids)))

					for _, id := range ids {
						if err := p.store.Delete(id); err != nil {
							log.Error("Failed to delete dropped transaction", "id", id, "err", err)
						}
					}
					break
				}
			}
		}
	}
	log.Debug("Blobpool tip threshold updated", "tip", tip)
	pooltipGauge.Update(tip.Int64())
	p.updateStorageMetrics()
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (p *BlobPool) validateTx(tx *action.SealedEnvelope) error {
	// TODO: Ensure the transaction adheres to basic pool filters (type, size, tip) and
	// consensus rules

	// TODO: If the transaction replaces an existing one, ensure that price bumps are
	// adhered to.
	var (
		from = common.BytesToAddress(tx.SenderAddress().Bytes()) // already validated above
		next = p.state.GetNonce(from)
	)
	if uint64(len(p.index[from])) > tx.Nonce()-next {
		// Account can support the replacement, but the price bump must also be met
		prev := p.index[from][int(tx.Nonce()-next)]
		switch {
		case GasFeeCapIntCmp(tx, prev.execFeeCap.ToBig()) <= 0:
			return fmt.Errorf("%w: new tx gas fee cap %v <= %v queued", txpool.ErrReplaceUnderpriced, tx.GasFeeCap(), prev.execFeeCap)
		case GasTipCapIntCmp(tx, prev.execTipCap.ToBig()) <= 0:
			return fmt.Errorf("%w: new tx gas tip cap %v <= %v queued", txpool.ErrReplaceUnderpriced, tx.GasTipCap(), prev.execTipCap)
		case BlobGasFeeCapIntCmp(tx, prev.blobFeeCap.ToBig()) <= 0:
			return fmt.Errorf("%w: new tx blob gas fee cap %v <= %v queued", txpool.ErrReplaceUnderpriced, BlobGasFeeCap(tx), prev.blobFeeCap)
		}
		var (
			multiplier = uint256.NewInt(100 + p.config.PriceBump)
			onehundred = uint256.NewInt(100)

			minGasFeeCap     = new(uint256.Int).Div(new(uint256.Int).Mul(multiplier, prev.execFeeCap), onehundred)
			minGasTipCap     = new(uint256.Int).Div(new(uint256.Int).Mul(multiplier, prev.execTipCap), onehundred)
			minBlobGasFeeCap = new(uint256.Int).Div(new(uint256.Int).Mul(multiplier, prev.blobFeeCap), onehundred)
		)
		switch {
		case GasFeeCapIntCmp(tx, minGasFeeCap.ToBig()) < 0:
			return fmt.Errorf("%w: new tx gas fee cap %v <= %v queued + %d%% replacement penalty", txpool.ErrReplaceUnderpriced, tx.GasFeeCap(), prev.execFeeCap, p.config.PriceBump)
		case GasTipCapIntCmp(tx, minGasTipCap.ToBig()) < 0:
			return fmt.Errorf("%w: new tx gas tip cap %v <= %v queued + %d%% replacement penalty", txpool.ErrReplaceUnderpriced, tx.GasTipCap(), prev.execTipCap, p.config.PriceBump)
		case BlobGasFeeCapIntCmp(tx, minBlobGasFeeCap.ToBig()) < 0:
			return fmt.Errorf("%w: new tx blob gas fee cap %v <= %v queued + %d%% replacement penalty", txpool.ErrReplaceUnderpriced, BlobGasFeeCap(tx), prev.blobFeeCap, p.config.PriceBump)
		}
	}
	return nil
}

// Has returns an indicator whether subpool has a transaction cached with the
// given hash.
func (p *BlobPool) Has(hash common.Hash) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	_, ok := p.lookup[hash]
	return ok
}

// Get returns a transaction if it is contained in the pool, or nil otherwise.
func (p *BlobPool) Get(hash common.Hash) *action.SealedEnvelope {
	// Track the amount of time waiting to retrieve a fully resolved blob tx from
	// the pool and the amount of time actually spent on pulling the data from disk.
	getStart := time.Now()
	p.lock.RLock()
	getwaitHist.Update(time.Since(getStart).Nanoseconds())
	defer p.lock.RUnlock()

	defer func(start time.Time) {
		gettimeHist.Update(time.Since(start).Nanoseconds())
	}(time.Now())

	// Pull the blob from disk and return an assembled response
	id, ok := p.lookup[hash]
	if !ok {
		return nil
	}
	data, err := p.store.Get(id)
	if err != nil {
		log.Error("Tracked blob transaction missing from store", "hash", hash, "id", id, "err", err)
		return nil
	}
	item := new(action.SealedEnvelope)
	if err = DecodeAction(data, item); err != nil {
		log.Error("Blobs corrupted for traced transaction", "hash", hash, "id", id, "err", err)
		return nil
	}
	return item
}

// Add inserts a set of blob transactions into the pool if they pass validation (both
// consensus validity and pool restrictions).
func (p *BlobPool) Add(txs []*action.SealedEnvelope, local bool, sync bool) []error {
	var (
		adds = make([]*action.SealedEnvelope, 0, len(txs))
		errs = make([]error, len(txs))
	)
	for i, tx := range txs {
		errs[i] = p.add(tx)
		if errs[i] == nil {
			adds = append(adds, WithoutBlobTxSidecar(tx))
		}
	}
	return errs
}

// Add inserts a new blob transaction into the pool if it passes validation (both
// consensus validity and pool restrictions).
func (p *BlobPool) add(tx *action.SealedEnvelope) (err error) {
	// The blob pool blocks on adding a transaction. This is because blob txs are
	// only even pulled form the network, so this method will act as the overload
	// protection for fetches.
	waitStart := time.Now()
	p.lock.Lock()
	addwaitHist.Update(time.Since(waitStart).Nanoseconds())
	defer p.lock.Unlock()

	defer func(start time.Time) {
		addtimeHist.Update(time.Since(start).Nanoseconds())
	}(time.Now())

	// Ensure the transaction is valid from all perspectives
	if err := p.validateTx(tx); err != nil {
		h, _ := tx.Hash()
		log.Trace("Transaction validation failed", "hash", common.BytesToHash(h[:]), "err", err)
		switch {
		case errors.Is(err, txpool.ErrUnderpriced):
			addUnderpricedMeter.Mark(1)
		case errors.Is(err, core.ErrNonceTooLow):
			addStaleMeter.Mark(1)
		case errors.Is(err, core.ErrNonceTooHigh):
			addGappedMeter.Mark(1)
		case errors.Is(err, core.ErrInsufficientFunds):
			addOverdraftedMeter.Mark(1)
		case errors.Is(err, txpool.ErrAccountLimitExceeded):
			addOvercappedMeter.Mark(1)
		case errors.Is(err, txpool.ErrReplaceUnderpriced):
			addNoreplaceMeter.Mark(1)
		default:
			addInvalidMeter.Mark(1)
		}
		return err
	}
	// If the address is not yet known, request exclusivity to track the account
	// only by this subpool until all transactions are evicted
	from := common.BytesToAddress(tx.SenderAddress().Bytes()) // already validated above
	if _, ok := p.index[from]; !ok {
		if err := p.reserve(from, true); err != nil {
			addNonExclusiveMeter.Mark(1)
			return err
		}
		defer func() {
			// If the transaction is rejected by some post-validation check, remove
			// the lock on the reservation set.
			//
			// Note, `err` here is the named error return, which will be initialized
			// by a return statement before running deferred methods. Take care with
			// removing or subscoping err as it will break this clause.
			if err != nil {
				p.reserve(from, false)
			}
		}()
	}
	// Transaction permitted into the pool from a nonce and cost perspective,
	// insert it into the database and update the indices
	blob, err := rlp.EncodeToBytes(tx)
	if err != nil {
		h, _ := tx.Hash()
		log.Error("Failed to encode transaction for storage", "hash", common.BytesToHash(h[:]), "err", err)
		return err
	}
	id, err := p.store.Put(blob)
	if err != nil {
		return err
	}
	meta := newBlobTxMeta(id, p.store.Size(id), tx)

	var (
		next   = p.state.GetNonce(from)
		offset = int(tx.Nonce() - next)
		newacc = false
	)
	var oldEvictionExecFeeJumps, oldEvictionBlobFeeJumps float64
	if txs, ok := p.index[from]; ok {
		oldEvictionExecFeeJumps = txs[len(txs)-1].evictionExecFeeJumps
		oldEvictionBlobFeeJumps = txs[len(txs)-1].evictionBlobFeeJumps
	}
	if len(p.index[from]) > offset {
		// Transaction replaces a previously queued one
		dropReplacedMeter.Mark(1)

		prev := p.index[from][offset]
		if err := p.store.Delete(prev.id); err != nil {
			// Shitty situation, but try to recover gracefully instead of going boom
			log.Error("Failed to delete replaced transaction", "id", prev.id, "err", err)
		}
		// Update the transaction index
		p.index[from][offset] = meta
		p.spent[from] = new(uint256.Int).Sub(p.spent[from], prev.costCap)
		p.spent[from] = new(uint256.Int).Add(p.spent[from], meta.costCap)

		delete(p.lookup, prev.hash)
		p.lookup[meta.hash] = meta.id
		p.stored += uint64(meta.size) - uint64(prev.size)
	} else {
		// Transaction extends previously scheduled ones
		p.index[from] = append(p.index[from], meta)
		if _, ok := p.spent[from]; !ok {
			p.spent[from] = new(uint256.Int)
			newacc = true
		}
		p.spent[from] = new(uint256.Int).Add(p.spent[from], meta.costCap)
		p.lookup[meta.hash] = meta.id
		p.stored += uint64(meta.size)
	}
	// Recompute the rolling eviction fields. In case of a replacement, this will
	// recompute all subsequent fields. In case of an append, this will only do
	// the fresh calculation.
	txs := p.index[from]

	for i := offset; i < len(txs); i++ {
		// The first transaction will always use itself
		if i == 0 {
			txs[0].evictionExecTip = txs[0].execTipCap
			txs[0].evictionExecFeeJumps = txs[0].basefeeJumps
			txs[0].evictionBlobFeeJumps = txs[0].blobfeeJumps

			continue
		}
		// Subsequent transactions will use a rolling calculation
		txs[i].evictionExecTip = txs[i-1].evictionExecTip
		if txs[i].evictionExecTip.Cmp(txs[i].execTipCap) > 0 {
			txs[i].evictionExecTip = txs[i].execTipCap
		}
		txs[i].evictionExecFeeJumps = txs[i-1].evictionExecFeeJumps
		if txs[i].evictionExecFeeJumps > txs[i].basefeeJumps {
			txs[i].evictionExecFeeJumps = txs[i].basefeeJumps
		}
		txs[i].evictionBlobFeeJumps = txs[i-1].evictionBlobFeeJumps
		if txs[i].evictionBlobFeeJumps > txs[i].blobfeeJumps {
			txs[i].evictionBlobFeeJumps = txs[i].blobfeeJumps
		}
	}
	// Update the eviction heap with the new information:
	//   - If the transaction is from a new account, add it to the heap
	//   - If the account had a singleton tx replaced, update the heap (new price caps)
	//   - If the account has a transaction replaced or appended, update the heap if significantly changed
	switch {
	case newacc:
		heap.Push(p.evict, from)

	case len(txs) == 1: // 1 tx and not a new acc, must be replacement
		heap.Fix(p.evict, p.evict.index[from])

	default: // replacement or new append
		evictionExecFeeDiff := oldEvictionExecFeeJumps - txs[len(txs)-1].evictionExecFeeJumps
		evictionBlobFeeDiff := oldEvictionBlobFeeJumps - txs[len(txs)-1].evictionBlobFeeJumps

		if math.Abs(evictionExecFeeDiff) > 0.001 || math.Abs(evictionBlobFeeDiff) > 0.001 { // need math.Abs, can go up and down
			heap.Fix(p.evict, p.evict.index[from])
		}
	}
	// If the pool went over the allowed data limit, evict transactions until
	// we're again below the threshold
	for p.stored > p.config.Datacap {
		p.drop()
	}
	p.updateStorageMetrics()

	addValidMeter.Mark(1)
	return nil
}

// drop removes the worst transaction from the pool. It is primarily used when a
// freshly added transaction overflows the pool and needs to evict something. The
// method is also called on startup if the user resizes their storage, might be an
// expensive run but it should be fine-ish.
func (p *BlobPool) drop() {
	// Peek at the account with the worse transaction set to evict from (Go's heap
	// stores the minimum at index zero of the heap slice) and retrieve it's last
	// transaction.
	var (
		from = p.evict.addrs[0] // cannot call drop on empty pool

		txs  = p.index[from]
		drop = txs[len(txs)-1]
		last = len(txs) == 1
	)
	// Remove the transaction from the pool's index
	if last {
		delete(p.index, from)
		delete(p.spent, from)
		p.reserve(from, false)
	} else {
		txs[len(txs)-1] = nil
		txs = txs[:len(txs)-1]

		p.index[from] = txs
		p.spent[from] = new(uint256.Int).Sub(p.spent[from], drop.costCap)
	}
	p.stored -= uint64(drop.size)
	delete(p.lookup, drop.hash)

	// Remove the transaction from the pool's eviction heap:
	//   - If the entire account was dropped, pop off the address
	//   - Otherwise, if the new tail has better eviction caps, fix the heap
	if last {
		heap.Pop(p.evict)
	} else {
		tail := txs[len(txs)-1] // new tail, surely exists

		evictionExecFeeDiff := tail.evictionExecFeeJumps - drop.evictionExecFeeJumps
		evictionBlobFeeDiff := tail.evictionBlobFeeJumps - drop.evictionBlobFeeJumps

		if evictionExecFeeDiff > 0.001 || evictionBlobFeeDiff > 0.001 { // no need for math.Abs, monotonic decreasing
			heap.Fix(p.evict, 0)
		}
	}
	// Remove the transaction from the data store
	log.Debug("Evicting overflown blob transaction", "from", from, "evicted", drop.nonce, "id", drop.id)
	dropOverflownMeter.Mark(1)

	if err := p.store.Delete(drop.id); err != nil {
		log.Error("Failed to drop evicted transaction", "id", drop.id, "err", err)
	}
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce.
func (p *BlobPool) Pending(enforceTips bool) map[common.Address][]*actpool.LazyTransaction {
	// Track the amount of time waiting to retrieve the list of pending blob txs
	// from the pool and the amount of time actually spent on assembling the data.
	// The latter will be pretty much moot, but we've kept it to have symmetric
	// across all user operations.
	pendStart := time.Now()
	p.lock.RLock()
	pendwaitHist.Update(time.Since(pendStart).Nanoseconds())
	defer p.lock.RUnlock()

	defer func(start time.Time) {
		pendtimeHist.Update(time.Since(start).Nanoseconds())
	}(time.Now())

	pending := make(map[common.Address][]*actpool.LazyTransaction)
	for addr, txs := range p.index {
		var lazies []*actpool.LazyTransaction
		for _, tx := range txs {
			lazies = append(lazies, &actpool.LazyTransaction{
				Pool:      p,
				Hash:      tx.hash,
				Time:      time.Now(), // TODO(karalabe): Maybe save these and use that?
				GasFeeCap: tx.execFeeCap.ToBig(),
				GasTipCap: tx.execTipCap.ToBig(),
				Gas:       tx.execGas,
				BlobGas:   tx.blobGas,
			})
		}
		if len(lazies) > 0 {
			pending[addr] = lazies
		}
	}
	return pending
}

// updateStorageMetrics retrieves a bunch of stats from the data store and pushes
// them out as metrics.
func (p *BlobPool) updateStorageMetrics() {
	stats := p.store.Infos()

	var (
		dataused uint64
		datareal uint64
		slotused uint64

		oversizedDataused uint64
		oversizedDatagaps uint64
		oversizedSlotused uint64
		oversizedSlotgaps uint64
	)
	for _, shelf := range stats.Shelves {
		slotDataused := shelf.FilledSlots * uint64(shelf.SlotSize)
		slotDatagaps := shelf.GappedSlots * uint64(shelf.SlotSize)

		dataused += slotDataused
		datareal += slotDataused + slotDatagaps
		slotused += shelf.FilledSlots

		metrics.GetOrRegisterGauge(fmt.Sprintf(shelfDatausedGaugeName, shelf.SlotSize/blobSize), nil).Update(int64(slotDataused))
		metrics.GetOrRegisterGauge(fmt.Sprintf(shelfDatagapsGaugeName, shelf.SlotSize/blobSize), nil).Update(int64(slotDatagaps))
		metrics.GetOrRegisterGauge(fmt.Sprintf(shelfSlotusedGaugeName, shelf.SlotSize/blobSize), nil).Update(int64(shelf.FilledSlots))
		metrics.GetOrRegisterGauge(fmt.Sprintf(shelfSlotgapsGaugeName, shelf.SlotSize/blobSize), nil).Update(int64(shelf.GappedSlots))

		if shelf.SlotSize/blobSize > maxBlobsPerTransaction {
			oversizedDataused += slotDataused
			oversizedDatagaps += slotDatagaps
			oversizedSlotused += shelf.FilledSlots
			oversizedSlotgaps += shelf.GappedSlots
		}
	}
	datausedGauge.Update(int64(dataused))
	datarealGauge.Update(int64(datareal))
	slotusedGauge.Update(int64(slotused))

	oversizedDatausedGauge.Update(int64(oversizedDataused))
	oversizedDatagapsGauge.Update(int64(oversizedDatagaps))
	oversizedSlotusedGauge.Update(int64(oversizedSlotused))
	oversizedSlotgapsGauge.Update(int64(oversizedSlotgaps))
}

// Nonce returns the next nonce of an account, with all transactions executable
// by the pool already applied on top.
func (p *BlobPool) Nonce(addr common.Address) uint64 {
	p.lock.Lock()
	defer p.lock.Unlock()

	if txs, ok := p.index[addr]; ok {
		return txs[len(txs)-1].nonce + 1
	}
	return p.state.GetNonce(addr)
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (p *BlobPool) Stats() (int, int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	var pending int
	for _, txs := range p.index {
		pending += len(txs)
	}
	return pending, 0 // No non-executable txs in the blob pool
}
