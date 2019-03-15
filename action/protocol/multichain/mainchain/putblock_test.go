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
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
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
	key2 := testaddress.Keyinfo["echo"]

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(
		ws,
		addr.String(),
		big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(unit.Iotx)),
	)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			Caller:   testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	defer func() {
		require.NoError(t, sf.Stop(ctx))
		ctrl.Finish()
	}()

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)

	p := NewProtocol(chain)

	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	pb := action.NewPutBlock(
		1,
		addr.String(),
		10001,
		roots,
		10003,
		big.NewInt(10004),
	)

	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetGasLimit(10003).
		SetAction(pb).Build()
	selp, err := action.Sign(elp, key2.PriKey)
	require.NoError(t, err)

	// first put
	_, err = p.Handle(ctx, selp.Action(), ws)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	// alredy exist
	_, err = p.Handle(ctx, selp.Action(), ws)
	require.Error(t, err)

	// get exist
	bp, exist := p.getBlockProof(pb.SubChainAddress(), pb.Height())
	require.True(t, exist)
	assert.Equal(t, bp.Height, pb.Height())
	assert.Equal(t, bp.SubChainAddress, pb.SubChainAddress())
	assert.Equal(t, bp.Roots[0].Name, "10002")
	assert.Equal(t, bp.Roots[0].Value, roots["10002"])

	// put new one
	roots["10002"] = hash.BytesToHash256([]byte("10003"))
	pb2 := action.NewPutBlock(
		1,
		addr.String(),
		10002,
		roots,
		10003,
		big.NewInt(10004),
	)

	elp = bd.SetNonce(1).
		SetGasLimit(10003).
		SetAction(pb2).Build()
	selp, err = action.Sign(elp, key2.PriKey)
	require.NoError(t, err)

	_, err = p.Handle(ctx, elp.Action(), ws)
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
