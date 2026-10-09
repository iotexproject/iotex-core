// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// TestActPool_blobCapEnforcedByWorker submits more blob txs than the
// per-account cap while the sender's worker is not draining its queue, so all
// of them pass the caller-side check before any is counted. The worker must
// still enforce the cap when it handles them.
func TestActPool_blobCapEnforcedByWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(big.NewInt(1_000_000_000_000)))
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())

	sk := identityset.PrivateKey(1)
	sender := sk.PublicKey().Address()
	recipient := identityset.Address(2).String()
	idx := ap.allocatedWorker(sender)
	worker := ap.worker[idx]
	// detach the sender's job queue from its worker so jobs pile up
	queue := make(chan workerJob, 2*_maxNumBlobTxPerAcct)
	ap.jobQueue[idx] = queue

	total := _maxNumBlobTxPerAcct + 2
	errs := make([]chan error, total)
	for i := 0; i < total; i++ {
		tx := action.NewBlobTx(0, uint64(i+1), 100000, big.NewInt(1), big.NewInt(1), nil,
			action.NewBlobTxData(uint256.NewInt(1), []common.Hash{{byte(i + 1)}}, nil))
		selp, err := action.Sign(action.NewEnvelope(tx, action.NewTransfer(big.NewInt(1), recipient, nil)), sk)
		r.NoError(err)
		errs[i] = make(chan error, 1)
		go func(c chan error) { c <- ap.Add(ctx, selp) }(errs[i])
		r.Eventually(func() bool { return len(queue) == i+1 }, 5*time.Second, time.Millisecond)
	}
	for i := 0; i < total; i++ {
		job := <-queue
		job.err <- worker.Handle(job)
	}
	for i := 0; i < total; i++ {
		err := <-errs[i]
		if i < _maxNumBlobTxPerAcct {
			r.NoError(err)
		} else {
			r.ErrorIs(err, action.ErrNonceTooHigh)
		}
	}
	r.Len(ap.GetUnconfirmedActs(sender.String()), _maxNumBlobTxPerAcct)
}
