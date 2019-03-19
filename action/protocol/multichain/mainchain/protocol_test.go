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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestAddSubChainActions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := config.Default
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(t, bc.Start(ctx))
	_, err := bc.CreateState(
		testaddress.Addrinfo["producer"].String(),
		big.NewInt(0).Mul(big.NewInt(10000000000), big.NewInt(unit.Iotx)),
	)
	require.NoError(t, err)
	ap, err := actpool.NewActPool(bc, cfg.ActPool)
	require.NoError(t, err)
	p := NewProtocol(bc)
	ap.AddActionValidators(p)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	defer require.NoError(t, bc.Stop(ctx))

	startSubChain := action.NewStartSubChain(
		1,
		2,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(unit.Iotx)),
		110,
		10,
		uint64(1000),
		testutil.TestGasPrice,
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetGasLimit(10000).
		SetAction(startSubChain).
		SetGasPrice(testutil.TestGasPrice).
		Build()
	selp, err := action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	require.NoError(t, ap.Add(selp))

	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	putBlock := action.NewPutBlock(
		2,
		testaddress.Addrinfo["producer"].String(),
		10001,
		roots,
		10003,
		testutil.TestGasPrice,
	)
	bd = &action.EnvelopeBuilder{}
	pbelp := bd.SetNonce(2).
		SetGasPrice(testutil.TestGasPrice).
		SetAction(putBlock).
		SetGasLimit(10003).Build()
	pbselp, err := action.Sign(pbelp, testaddress.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	require.NoError(t, ap.Add(pbselp))

	stopSubChain := action.NewStopSubChain(
		3,
		testaddress.Addrinfo["alfa"].String(),
		10003,
		10005,
		testutil.TestGasPrice,
	)
	bd = &action.EnvelopeBuilder{}
	sscelp := bd.SetNonce(3).
		SetGasPrice(testutil.TestGasPrice).
		SetAction(stopSubChain).
		SetGasLimit(10005).Build()
	sscselp, err := action.Sign(sscelp, testaddress.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	require.NoError(t, ap.Add(sscselp))

	l := 0
	for _, part := range ap.PendingActionMap() {
		l += len(part)
	}
	assert.Equal(t, 3, l)
}
