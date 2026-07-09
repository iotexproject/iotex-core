// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func TestHelperContextRoundTrip(t *testing.T) {
	r := require.New(t)

	blackListed := func(addr string, height uint64) bool { return addr == "bad" && height == 7 }
	hctx := HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: nil,
		DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
		IsBlackListed: blackListed,
	}

	ctx := WithHelperCtx(context.Background(), hctx)
	got := mustGetHelperCtx(ctx)
	r.NotNil(got.GetBlockHash)
	r.NotNil(got.DepositGasFunc)
	r.True(got.IsBlackListed("bad", 7))
	r.False(got.IsBlackListed("bad", 8))
}

func TestMustGetHelperCtxPanicsWhenMissing(t *testing.T) {
	require.Panics(t, func() {
		mustGetHelperCtx(context.Background())
	})
}

func TestTracerContextRoundTrip(t *testing.T) {
	r := require.New(t)

	// absent tracer context reports not-ok
	_, ok := GetTracerCtx(context.Background())
	r.False(ok)

	var captured *action.Receipt
	tctx := TracerContext{
		CaptureTx: func(_ []byte, receipt *action.Receipt) { captured = receipt },
	}
	ctx := WithTracerCtx(context.Background(), tctx)
	got, ok := GetTracerCtx(ctx)
	r.True(ok)
	r.NotNil(got.CaptureTx)

	want := &action.Receipt{BlockHeight: 42}
	got.CaptureTx(nil, want)
	r.Equal(want, captured)
}
