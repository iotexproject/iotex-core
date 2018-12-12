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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestHandlePutBlock(t *testing.T) {
	t.Parallel()

	cfg := config.Default
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(sf).AnyTimes()

	addr := testaddress.Addrinfo["producer"]
	addr2 := testaddress.Addrinfo["echo"]

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(
		ws,
		addr.RawAddress,
		big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(blockchain.Iotx)),
	)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	defer func() {
		require.NoError(t, sf.Stop(ctx))
		ctrl.Finish()
	}()

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)

	p := NewProtocol(chain)

	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	pb := action.NewPutBlock(
		1,
		addr2.RawAddress,
		addr.RawAddress,
		10001,
		roots,
		10003,
		big.NewInt(10004),
	)

	// first put
	_, err = p.Handle(ctx, pb, ws)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	// alredy exist
	_, err = p.Handle(ctx, pb, ws)
	require.Error(t, err)

	// get exist
	bp, exist := p.getBlockProof(pb.SubChainAddress(), pb.Height())
	require.True(t, exist)
	assert.Equal(t, bp.Height, pb.Height())
	assert.Equal(t, bp.SubChainAddress, pb.SubChainAddress())
	assert.Equal(t, bp.Roots[0].Name, "10002")
	assert.Equal(t, bp.Roots[0].Value, roots["10002"])

	// put new one
	roots["10002"] = byteutil.BytesTo32B([]byte("10003"))
	pb2 := action.NewPutBlock(
		1,
		addr2.RawAddress,
		addr.RawAddress,
		10002,
		roots,
		10003,
		big.NewInt(10004),
	)
	_, err = p.Handle(ctx, pb2, ws)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	// get new one
	bp2, exist := p.getBlockProof(pb2.SubChainAddress(), pb2.Height())
	require.True(t, exist)
	assert.Equal(t, bp2.Height, pb2.Height())
	assert.Equal(t, bp2.SubChainAddress, pb2.SubChainAddress())
	assert.Equal(t, bp2.Roots[0].Name, "10002")
	assert.Equal(t, bp2.Roots[0].Value, roots["10002"])
}
