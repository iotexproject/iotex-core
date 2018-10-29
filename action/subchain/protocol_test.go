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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestAddSubChainActions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := config.Default
	bc := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(t, bc.Start(ctx))
	_, err := bc.CreateState(
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(10000000000), big.NewInt(blockchain.Iotx)),
	)
	require.NoError(t, err)
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	require.NoError(t, err)
	p := NewProtocol(&cfg, nil, nil, bc, nil)
	require.NoError(t, p.Start(ctx))
	ap.AddActionValidators(p)
	defer func() {
		require.NoError(t, p.Stop(ctx))
		require.NoError(t, bc.Stop(ctx))
	}()

	require.NoError(
		t,
		ap.Add(
			action.NewStartSubChain(
				1,
				2,
				testaddress.Addrinfo["producer"].RawAddress,
				MinSecurityDeposit,
				big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
				110,
				10,
				0,
				big.NewInt(0),
			),
		),
	)

	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	require.NoError(
		t,
		ap.Add(
			action.NewPutBlock(
				2,
				testaddress.Addrinfo["alfa"].RawAddress,
				testaddress.Addrinfo["producer"].RawAddress,
				10001,
				roots,
				10003,
				big.NewInt(10004),
			),
		),
	)

	require.NoError(
		t,
		ap.Add(
			action.NewStopSubChain(
				testaddress.Addrinfo["producer"].RawAddress,
				3,
				2,
				testaddress.Addrinfo["alfa"].RawAddress,
				10003,
				10005,
				big.NewInt(10006),
			),
		),
	)

	assert.Equal(t, 3, len(ap.PickActs()))
}
