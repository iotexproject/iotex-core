// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func TestTraceCancellerRegisterThenCancel(t *testing.T) {
	r := require.New(t)
	tc := NewTraceCanceller()
	var n atomic.Int32
	tc.register(func() { n.Add(1) })
	tc.register(func() { n.Add(1) })
	r.EqualValues(0, n.Load())
	tc.Cancel()
	r.EqualValues(2, n.Load())
	// idempotent: second Cancel must not re-invoke
	tc.Cancel()
	r.EqualValues(2, n.Load())
}

func TestTraceCancellerRegisterAfterFired(t *testing.T) {
	r := require.New(t)
	tc := NewTraceCanceller()
	tc.Cancel()
	var n atomic.Int32
	// registered after the watchdog fired -> cancelled immediately
	tc.register(func() { n.Add(1) })
	r.EqualValues(1, n.Load())
}

func TestTraceCancellerContext(t *testing.T) {
	r := require.New(t)
	r.Nil(GetTraceCanceller(context.Background()))
	tc := NewTraceCanceller()
	ctx := WithTraceCanceller(context.Background(), tc)
	r.Same(tc, GetTraceCanceller(ctx))
}

// TestTraceCancellerAbortsEVM proves the whole chain: an infinite-loop
// contract executed through ExecuteContract is aborted by TraceCanceller.Cancel
// instead of running until gas exhaustion.
func TestTraceCancellerAbortsEVM(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	sm.EXPECT().DelState(gomock.Any()).Return(uint64(0), nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()

	// deploy-style execution whose init code is an infinite loop:
	// JUMPDEST; PUSH1 0; JUMP  (0x5b600056)
	loop := []byte{0x5b, 0x60, 0x00, 0x56}
	gasLimit := uint64(100_000_000) // huge: uncancelled execution would spin for a long time
	e := action.NewExecution("", big.NewInt(0), loop)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(0)).
		SetGasLimit(gasLimit).SetAction(e).Build()

	g := genesis.TestDefault()
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller: identityset.Address(27),
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		Producer: identityset.Address(27),
		GasLimit: gasLimit,
	})
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 100,
	})
	ctx = WithHelperCtx(ctx, HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: func(uint64) (time.Time, error) { return time.Time{}, nil },
		DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
	})
	tc := NewTraceCanceller()
	ctx = WithTraceCanceller(ctx, tc)

	go func() {
		time.Sleep(100 * time.Millisecond)
		tc.Cancel()
	}()
	start := time.Now()
	_, receipt, err := ExecuteContract(ctx, sm, elp)
	elapsed := time.Since(start)
	r.NoError(err)
	r.NotNil(receipt)
	// abort halts via errStopToken (clean stop), so the deploy "succeeds"
	// without consuming the full gas budget; the tracer layer reports the
	// timeout error to the caller. The key assertions: it returned promptly
	// and did not burn the whole gas limit spinning.
	r.Less(elapsed, 10*time.Second, "execution was not aborted by Cancel")
	r.Less(receipt.GasConsumed, gasLimit, "aborted execution must not consume the full gas limit")
}
