// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"sync"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

const (
	_evPoolSize = 20
	_evGas      = uint64(100000)
	_evHeadGP   = 100 // gas price of the sender's executable head action
	_evQueuedGP = 1   // gas price of the sender's queued actions
	_evOtherGP  = 10  // gas price of other senders: above queued, below head
)

type evChain struct {
	mu    sync.Mutex
	nonce map[string]uint64 // confirmed nonce (number of mined txs)
	bal   map[string]*big.Int
}

func (c *evChain) set(addr address.Address, nonce uint64, bal *big.Int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nonce[string(addr.Bytes())] = nonce
	c.bal[string(addr.Bytes())] = new(big.Int).Set(bal)
}

type evEnv struct {
	r      *require.Assertions
	ap     *actPool
	ctx    context.Context
	chain  *evChain
	sender int
	others []int
}

func newEvEnv(t *testing.T) *evEnv {
	ctrl := gomock.NewController(t)
	r := require.New(t)
	chain := &evChain{nonce: map[string]uint64{}, bal: map[string]*big.Int{}}
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct := acc.(*state.Account)
		cfg := &protocol.StateConfig{}
		for _, o := range opts {
			o(cfg)
		}
		chain.mu.Lock()
		defer chain.mu.Unlock()
		key := string(cfg.Key)
		if b, ok := chain.bal[key]; ok {
			r.NoError(acct.AddBalance(new(big.Int).Set(b)))
		}
		for i := uint64(1); i <= chain.nonce[key]; i++ {
			r.NoError(acct.SetPendingNonce(i + 1))
		}
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	cfg.MaxNumActsPerPool = _evPoolSize
	cfg.BlackList = nil
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	e := &evEnv{
		r: r, ap: ap, chain: chain,
		ctx:    genesis.WithGenesisContext(context.Background(), genesis.TestDefault()),
		sender: 20,
		others: []int{21, 22, 23},
	}
	rich := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	for _, h := range e.others {
		chain.set(identityset.Address(h), 0, rich)
	}
	return e
}

func (e *evEnv) tx(idx int, nonce uint64, gp int64) *action.SealedEnvelope {
	selp, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(idx), nonce, big.NewInt(0), nil, _evGas, big.NewInt(gp))
	e.r.NoError(err)
	return selp
}

// cost returns the actpool-visible cost (intrinsic gas * price + value).
func (e *evEnv) cost(gp int64) *big.Int {
	c, err := e.tx(e.sender, 1, gp).Cost()
	e.r.NoError(err)
	return c
}

func (e *evEnv) senderQueue() ActQueue {
	addr := identityset.Address(e.sender)
	return e.ap.worker[e.ap.allocatedWorker(addr)].accountActs.Account(addr.String())
}

func (e *evEnv) inPool(selp *action.SealedEnvelope) bool {
	h, _ := selp.Hash()
	_, err := e.ap.GetActionByHash(h)
	return err == nil
}

func (e *evEnv) senderPending() int {
	return len(e.ap.PendingActionMap()[identityset.Address(e.sender).String()])
}

// Queued actions admitted before an executable head that spends the whole
// balance must not keep a full pool closed to other executable actions.
func TestActPool_EvictQueuedBeforeExecutable_Balance(t *testing.T) {
	e := newEvEnv(t)
	r := e.r
	sender := identityset.Address(e.sender)
	e.chain.set(sender, 0, e.cost(_evHeadGP)) // balance covers exactly the head

	for n := uint64(2); n <= _evPoolSize; n++ {
		r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, n, _evQueuedGP)), "nonce %d", n)
	}
	r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, 1, _evHeadGP)))
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())
	r.Equal(1, e.senderPending())
	r.Equal(_evPoolSize-1, e.senderQueue().NumQueuedActs())

	other := e.tx(e.others[0], 1, _evOtherGP)
	r.NoError(e.ap.Add(e.ctx, other))
	r.True(e.inPool(other))
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())
	r.Equal(_evPoolSize-1, e.senderQueue().Len(), "one queued action evicted")
	r.Equal(1, e.senderPending(), "executable head kept")
	r.False(e.inPool(e.tx(e.sender, _evPoolSize, _evQueuedGP)), "largest nonce evicted")
}

// Same as above with a nonce gap instead of insufficient balance.
func TestActPool_EvictQueuedBeforeExecutable_NonceGap(t *testing.T) {
	e := newEvEnv(t)
	r := e.r
	sender := identityset.Address(e.sender)
	rich := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	e.chain.set(sender, 0, rich)

	r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, 1, _evHeadGP)))
	for n := uint64(3); n <= _evPoolSize+1; n++ {
		r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, n, _evQueuedGP)), "nonce %d", n)
	}
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())
	r.Equal(1, e.senderPending())

	for i, idx := range e.others {
		other := e.tx(idx, 1, _evOtherGP)
		r.NoError(e.ap.Add(e.ctx, other))
		r.True(e.inPool(other))
		r.Equal(_evPoolSize-1-i, e.senderQueue().Len())
	}
	r.Equal(1, e.senderPending(), "executable head kept")
}

// A queued action arriving at a full pool is the one dropped when its sender
// holds the most queued actions; executable actions of others stay.
func TestActPool_EvictQueuedBeforeExecutable_NewQueuedDropped(t *testing.T) {
	e := newEvEnv(t)
	r := e.r
	rich := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	e.chain.set(identityset.Address(e.sender), 0, rich)

	// fill the pool with executable actions of other senders
	var others []*action.SealedEnvelope
	for n := uint64(1); n <= _evPoolSize; n++ {
		act := e.tx(e.others[int(n)%len(e.others)], (n-1)/uint64(len(e.others))+1, _evOtherGP)
		r.NoError(e.ap.Add(e.ctx, act))
		others = append(others, act)
	}
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())

	queued := e.tx(e.sender, 2, _evHeadGP)
	_ = e.ap.Add(e.ctx, queued)
	r.False(e.inPool(queued))
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())
	for _, act := range others {
		r.True(e.inPool(act))
	}
}

// Re-adding one executable head per block does not keep the pool closed.
func TestActPool_EvictQueuedBeforeExecutable_AcrossBlocks(t *testing.T) {
	e := newEvEnv(t)
	r := e.r
	sender := identityset.Address(e.sender)
	rich := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)
	e.chain.set(sender, 0, rich)

	r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, 1, _evHeadGP)))
	for n := uint64(10); n < 10+_evPoolSize-1; n++ {
		r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, n, _evQueuedGP)))
	}
	r.Equal(uint64(_evPoolSize), e.ap.GetSize())

	for blk := uint64(1); blk <= 3; blk++ {
		other := e.tx(e.others[0], blk, _evOtherGP)
		r.NoError(e.ap.Add(e.ctx, other), "block %d", blk)
		r.True(e.inPool(other))

		// block mined: the sender's head and the other action are confirmed
		e.chain.set(sender, blk, rich)
		e.chain.set(identityset.Address(e.others[0]), blk, rich)
		e.ap.Reset()
		r.NoError(e.ap.Add(e.ctx, e.tx(e.sender, blk+1, _evHeadGP)))
	}
}
