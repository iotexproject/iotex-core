// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestHandleStopSubChain(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sender := testaddress.Addrinfo["producer"]
	factory := mock_state.NewMockFactory(ctrl)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(factory).AnyTimes()
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()

	ws := mock_state.NewMockWorkingSet(ctrl)
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&state.SortedSlice{InOperation{ID: uint32(2)}}, nil).
		Times(1)
	ws.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).Times(6)
	ws.EXPECT().
		CachedAccountState(gomock.Any()).
		Return(&state.Account{
			Nonce:   2,
			Balance: big.NewInt(400000000),
		}, nil).
		Times(3)
	ws.EXPECT().Height().Return(uint64(2)).Times(5)
	subChainPKHash, err := createSubChainAddress(sender.RawAddress, 2)
	require.NoError(err)
	subChain := SubChain{
		ChainID:            2,
		SecurityDeposit:    big.NewInt(200000),
		OperationDeposit:   big.NewInt(200000),
		StartHeight:        3,
		ParentHeightOffset: 1,
		OwnerPublicKey:     sender.PublicKey,
		CurrentHeight:      0,
		DepositCount:       0,
	}
	factory.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&subChain, nil).
		Times(3)
	subChainAddr := address.New(chain.ChainID(), subChainPKHash[:])

	p := NewProtocol(chain)
	stop := action.NewStopSubChain(
		testaddress.Addrinfo["alfa"].RawAddress,
		uint64(5),
		subChainAddr.IotxAddress(),
		uint64(10),
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(action.Sign(stop, testaddress.Addrinfo["alfa"].PrivateKey))
	// wrong owner
	require.Error(p.handleStopSubChain(stop, ws))
	stop = action.NewStopSubChain(
		sender.RawAddress,
		uint64(5),
		subChainAddr.IotxAddress(),
		uint64(1),
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(action.Sign(stop, sender.PrivateKey))
	// wrong stop height
	require.Error(p.handleStopSubChain(stop, ws))
	stop = action.NewStopSubChain(
		sender.RawAddress,
		uint64(5),
		subChainAddr.IotxAddress(),
		uint64(10),
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(action.Sign(stop, sender.PrivateKey))
	require.NoError(p.handleStopSubChain(stop, ws))

	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&state.SortedSlice{}, nil).
		Times(1)
	// not sub-chain in operation
	require.Error(p.handleStopSubChain(stop, ws))
}
