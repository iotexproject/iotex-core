// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestHandlePutBlock(t *testing.T) {
	t.Parallel()

	cfg := config.Default
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(sf).AnyTimes()

	defer func() {
		require.NoError(t, sf.Stop(ctx))
		ctrl.Finish()
	}()

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)

	p := NewProtocol(&cfg, nil, nil, chain, nil)

	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	addr := testaddress.Addrinfo["producer"]
	addr2 := testaddress.Addrinfo["echo"]
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
	require.NoError(t, p.Handle(pb, ws))
	require.NoError(t, sf.Commit(ws))

	// alredy exist
	require.Error(t, p.Handle(pb, ws))

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
	require.NoError(t, p.Handle(pb2, ws))
	require.NoError(t, sf.Commit(ws))

	// get new one
	bp2, exist := p.getBlockProof(pb2.SubChainAddress(), pb2.Height())
	require.True(t, exist)
	assert.Equal(t, bp2.Height, pb2.Height())
	assert.Equal(t, bp2.SubChainAddress, pb2.SubChainAddress())
	assert.Equal(t, bp2.Roots[0].Name, "10002")
	assert.Equal(t, bp2.Roots[0].Value, roots["10002"])
}
