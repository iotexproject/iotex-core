// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
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

func TestChainService_RejectsLossyTxContainer(t *testing.T) {
	r := require.New(t)
	const evmNetworkID = 4689
	unstake, err := action.NewUnstake(1, nil).EthData()
	r.NoError(err)
	// a staking call carrying value does not survive the conversion to a native action
	to := common.BytesToAddress(address.StakingProtocolAddrHash[:])
	tx := types.MustSignNewTx(identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey),
		types.LatestSignerForChainID(big.NewInt(evmNetworkID)), &types.DynamicFeeTx{
			ChainID: big.NewInt(evmNetworkID), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
			Gas: 100000, To: &to, Value: big.NewInt(1), Data: unstake,
		})
	raw, err := tx.MarshalBinary()
	r.NoError(err)
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	r.NoError(err)
	pb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			ChainID: 1,
			Action:  &iotextypes.ActionCore_TxContainer{TxContainer: &iotextypes.TxContainer{Raw: raw}},
		},
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	}

	newChainService := func(t *testing.T) *ChainService {
		ctrl := gomock.NewController(t)
		chain := mock_blockchain.NewMockBlockchain(ctrl)
		chain.EXPECT().EvmNetworkID().Return(uint32(evmNetworkID)).AnyTimes()
		ap := mock_actpool.NewMockActPool(ctrl)
		ap.EXPECT().BundlePool().Return(actpool.NewBundlePool(genesis.TestDefault())).AnyTimes()
		// no Add expected: the pool never sees the action
		return &ChainService{chain: chain, actpool: ap}
	}
	t.Run("HandleAction", func(t *testing.T) {
		require.ErrorIs(t, newChainService(t).HandleAction(context.Background(), pb), action.ErrInvalidAct)
	})
	t.Run("HandleBundle", func(t *testing.T) {
		bundle := &iotextypes.Bundle{Actions: []*iotextypes.Action{pb}}
		require.ErrorIs(t, newChainService(t).HandleBundle(context.Background(), bundle), action.ErrInvalidAct)
	})
}
