// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
)

func TestChainService_Filter_NilHeader(t *testing.T) {
	r := require.New(t)
	cs := &ChainService{}

	cases := []struct {
		name string
		msg  *iotextypes.Block
	}{
		{"nil block", nil},
		{"nil header", &iotextypes.Block{Header: nil}},
		{"nil header core", &iotextypes.Block{Header: &iotextypes.BlockHeader{Core: nil}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.NotPanics(func() {
				r.False(cs.Filter(iotexrpc.MessageType_BLOCK, tc.msg, 10))
			})
		})
	}

	// Non-BLOCK messages always pass without inspecting payload.
	r.True(cs.Filter(iotexrpc.MessageType_ACTION, nil, 10))
}

func TestChainService_ReportFullness_NilHeader(t *testing.T) {
	r := require.New(t)
	cs := &ChainService{}

	cases := []struct {
		name string
		msg  *iotextypes.Block
	}{
		{"nil block", nil},
		{"nil header", &iotextypes.Block{Header: nil}},
		{"nil header core", &iotextypes.Block{Header: &iotextypes.BlockHeader{Core: nil}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.NotPanics(func() {
				cs.ReportFullness(nil, iotexrpc.MessageType_BLOCK, tc.msg, 0.5)
			})
		})
	}
}

func TestStartSupplyObserver(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	// Empty ChainService (no factory): observer is a no-op and not started.
	cs := &ChainService{}
	r.False(cs.startSupplyObserver(ctx))
	r.Nil(cs.supplyObserver)

	// With a state factory, the observer is constructed (from the chain genesis)
	// and started.
	ctrl := gomock.NewController(t)
	factory := mock_factory.NewMockFactory(ctrl)
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().Genesis().Return(genesis.TestDefault()).AnyTimes()

	cs2 := &ChainService{factory: factory, chain: bc}
	r.True(cs2.startSupplyObserver(ctx))
	r.NotNil(cs2.supplyObserver)

	// A second call must not recreate the observer; it just restarts it.
	r.True(cs2.startSupplyObserver(ctx))
	r.NotNil(cs2.supplyObserver)
}
