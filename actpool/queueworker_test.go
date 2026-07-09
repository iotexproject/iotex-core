// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// newTestWorkerActPool builds an actPool backed by a mock state reader that is
// convenient for direct queueWorker unit tests.
func newTestWorkerActPool(t *testing.T) (*actPool, *mock_chainmanager.MockStateReader) {
	t.Helper()
	ctrl := gomock.NewController(t)
	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	Ap, err := NewActPool(genesis.TestDefault(), sf, getActPoolCfg())
	require.NoError(t, err)
	return Ap.(*actPool), sf
}

func TestQueueWorker_NewAndStartValidation(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)

	t.Run("newQueueWorker initializes sub-structures", func(t *testing.T) {
		w := newQueueWorker(ap, make(chan workerJob, 1))
		r.NotNil(w.accountActs)
		r.NotNil(w.emptyAccounts)
		r.Equal(ap, w.ap)
	})
	t.Run("Start rejects an invalid worker", func(t *testing.T) {
		r.Error((&queueWorker{ap: ap}).Start())                      // nil queue
		r.Error((&queueWorker{queue: make(chan workerJob)}).Start()) // nil ap
	})
}

// TestQueueWorker_Lifecycle drives the worker goroutine deterministically: a job
// carrying a canceled context is processed and its error is surfaced through the
// job's err channel, then Stop closes the queue and the goroutine returns.
func TestQueueWorker_Lifecycle(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)
	ch := make(chan workerJob, 1)
	w := newQueueWorker(ap, ch)
	r.NoError(w.Start())

	tsf, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Handle must return ctx.Err() before touching any state
	errCh := make(chan error, 1)
	ch <- workerJob{ctx: ctx, act: tsf, rep: false, err: errCh}
	r.ErrorIs(<-errCh, context.Canceled)

	r.NoError(w.Stop())
}

func TestQueueWorker_CheckSelpWithState(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)
	w := ap.worker[0]

	t.Run("nonce below pending is rejected", func(t *testing.T) {
		act, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		r.ErrorIs(w.checkSelpWithState(act, 5, big.NewInt(maxBalance)), action.ErrNonceTooLow)
	})
	t.Run("nonce beyond per-account window is rejected", func(t *testing.T) {
		// gap of exactly MaxNumActsPerAcct is out of range
		nonce := 1 + ap.cfg.MaxNumActsPerAcct
		act, err := action.SignedTransfer(_addr2, _priKey1, nonce, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		r.ErrorIs(w.checkSelpWithState(act, 1, big.NewInt(maxBalance)), action.ErrNonceTooHigh)
	})
	t.Run("insufficient balance is rejected", func(t *testing.T) {
		act, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.ErrorIs(w.checkSelpWithState(act, 1, big.NewInt(1)), action.ErrInsufficientFunds)
	})
	t.Run("valid action passes", func(t *testing.T) {
		act, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(w.checkSelpWithState(act, 1, big.NewInt(maxBalance)))
		// the boundary just inside the window is accepted
		nonce := ap.cfg.MaxNumActsPerAcct // gap of MaxNumActsPerAcct-1 from pending nonce 1
		act, err = action.SignedTransfer(_addr2, _priKey1, nonce, big.NewInt(100), nil, uint64(0), big.NewInt(1))
		r.NoError(err)
		r.NoError(w.checkSelpWithState(act, 1, big.NewInt(maxBalance)))
	})
}

func TestQueueWorker_GetConfirmedState(t *testing.T) {
	r := require.New(t)

	t.Run("served from cached queue without hitting state reader", func(t *testing.T) {
		ap, _ := newTestWorkerActPool(t)
		w := ap.worker[0]
		tsf, err := action.SignedTransfer(_addr2, _priKey1, 3, big.NewInt(1), nil, uint64(0), big.NewInt(0))
		r.NoError(err)
		// seed a queue for _addr1 with confirmed nonce 3 / balance 777
		r.NoError(w.putAction(_addr1, tsf, 3, big.NewInt(777)))
		addr, err := address.FromString(_addr1)
		r.NoError(err)
		nonce, balance, err := w.getConfirmedState(context.Background(), addr)
		r.NoError(err)
		r.Equal(uint64(3), nonce)
		r.Equal(big.NewInt(777), balance)
	})

	t.Run("falls back to state reader when uncached", func(t *testing.T) {
		ap, sf := newTestWorkerActPool(t)
		w := ap.worker[0]
		sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acct interface{}, opts ...protocol.StateOption) (uint64, error) {
			a := acct.(*state.Account)
			r.NoError(a.AddBalance(big.NewInt(9999)))
			r.NoError(a.SetPendingNonce(1)) // converts to zero-nonce type; pending nonce == 1 either way
			return 0, nil
		}).Times(1)
		ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(
			genesis.WithGenesisContext(context.Background(), genesis.TestDefault()),
			protocol.BlockCtx{BlockHeight: 1},
		))
		addr, err := address.FromString(_addr2)
		r.NoError(err)
		nonce, balance, err := w.getConfirmedState(ctx, addr)
		r.NoError(err)
		r.Equal(uint64(1), nonce)
		r.Equal(big.NewInt(9999), balance)
	})
}

func TestQueueWorker_PendingNonceAndAllActions(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)
	w := ap.worker[0]
	addr, err := address.FromString(_addr1)
	r.NoError(err)

	// unknown account
	_, ok := w.PendingNonce(addr)
	r.False(ok)
	acts, ok := w.AllActions(addr)
	r.Nil(acts)
	r.False(ok)

	// put two continuous actions from confirmed nonce 1
	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	r.NoError(w.putAction(_addr1, tsf1, 1, big.NewInt(maxBalance)))
	r.NoError(w.putAction(_addr1, tsf2, 1, big.NewInt(maxBalance)))

	nonce, ok := w.PendingNonce(addr)
	r.True(ok)
	r.Equal(uint64(3), nonce) // both actions committable -> pending nonce is 3
	acts, ok = w.AllActions(addr)
	r.True(ok)
	r.Equal([]*action.SealedEnvelope{tsf1, tsf2}, acts) // sorted ascending by nonce
}

func TestQueueWorker_ResetAccount(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)
	w := ap.worker[0]
	addr, err := address.FromString(_addr1)
	r.NoError(err)

	// resetting an unknown account is a no-op returning nil
	r.Nil(w.ResetAccount(addr))

	tsf1, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(_addr2, _priKey1, 2, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	r.NoError(w.putAction(_addr1, tsf1, 1, big.NewInt(maxBalance)))
	r.NoError(w.putAction(_addr1, tsf2, 1, big.NewInt(maxBalance)))

	pending := w.ResetAccount(addr)
	r.ElementsMatch([]*action.SealedEnvelope{tsf1, tsf2}, pending)
	// account is popped out of the pool and queued for empty cleanup
	r.Nil(w.accountActs.Account(_addr1))
	_, ok := w.emptyAccounts.Get(_addr1)
	r.True(ok)
}

func TestQueueWorker_RemoveEmptyAccounts(t *testing.T) {
	r := require.New(t)
	ap, _ := newTestWorkerActPool(t)
	w := ap.worker[0]

	// early return path: nothing marked empty
	w.removeEmptyAccounts()
	r.Equal(0, w.emptyAccounts.Count())

	tsf, err := action.SignedTransfer(_addr2, _priKey1, 1, big.NewInt(1), nil, uint64(0), big.NewInt(0))
	r.NoError(err)
	r.NoError(w.putAction(_addr1, tsf, 1, big.NewInt(maxBalance)))
	// drain the queue so the account becomes empty, then mark it
	r.Equal(tsf, w.accountActs.PopPeek())
	r.True(w.accountActs.Account(_addr1).Empty())
	w.emptyAccounts.Set(_addr1, struct{}{})

	w.removeEmptyAccounts()
	r.Nil(w.accountActs.Account(_addr1)) // deleted because empty
	r.Equal(0, w.emptyAccounts.Count())  // marker cache reset
}
