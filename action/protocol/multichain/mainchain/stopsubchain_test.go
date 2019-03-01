// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestHandleStopSubChain(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := testaddress.Addrinfo["producer"]
	factory := mock_factory.NewMockFactory(ctrl)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(factory).AnyTimes()
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()

	ws := mock_factory.NewMockWorkingSet(ctrl)
	ws.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).Times(6)
	ws.EXPECT().State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := &state.Account{Nonce: 2, Balance: big.NewInt(400000000)}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(2)
	ws.EXPECT().State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := SubChainsInOperation{InOperation{ID: uint32(2)}}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(1)
	ws.EXPECT().State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := &state.Account{Nonce: 2, Balance: big.NewInt(400000000)}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(1)
	subChainPKHash, err := createSubChainAddress(sender.String(), 2)
	require.NoError(err)
	subChain := &SubChain{
		ChainID:            2,
		SecurityDeposit:    big.NewInt(200000),
		OperationDeposit:   big.NewInt(200000),
		StartHeight:        3,
		ParentHeightOffset: 1,
		OwnerPublicKey:     testaddress.Keyinfo["producer"].PubKey,
		CurrentHeight:      0,
		DepositCount:       0,
	}
	factory.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			data, err := state.Serialize(subChain)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(3)
	subChainAddr, err := address.FromBytes(subChainPKHash[:])
	require.NoError(err)

	p := NewProtocol(chain)
	stop := action.NewStopSubChain(
		uint64(5),
		subChainAddr.String(),
		uint64(10),
		uint64(100000),
		big.NewInt(0),
	)
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:      testaddress.Addrinfo["alfa"],
		BlockHeight: 2,
	})
	// wrong owner
	require.Error(p.handleStopSubChain(ctx, stop, ws))
	stop = action.NewStopSubChain(
		uint64(5),
		subChainAddr.String(),
		uint64(1),
		uint64(100000),
		big.NewInt(0),
	)
	ctx = protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:      sender,
		BlockHeight: 2,
	})
	// wrong stop height
	require.Error(p.handleStopSubChain(ctx, stop, ws))
	stop = action.NewStopSubChain(
		uint64(5),
		subChainAddr.String(),
		uint64(10),
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(p.handleStopSubChain(ctx, stop, ws))

	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	// not sub-chain in operation
	require.Error(p.handleStopSubChain(ctx, stop, ws))
}
