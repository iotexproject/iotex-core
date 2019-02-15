// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol/account"

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state/factory"
)

func testProtocol(t *testing.T, test func(*testing.T, context.Context, factory.Factory, *Protocol)) {
	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer require.NoError(t, stateDB.Stop(context.Background()))

	sk, err := crypto.GenerateKey()
	require.NoError(t, err)
	pkHash := keypair.HashPubKey(&sk.PublicKey)
	addr, err := address.FromBytes(pkHash[:])
	require.NoError(t, err)

	skProducer, err := crypto.GenerateKey()
	require.NoError(t, err)
	pkHashProducer := keypair.HashPubKey(&skProducer.PublicKey)
	addrProducer, err := address.FromBytes(pkHashProducer[:])
	require.NoError(t, err)
	p := NewProtocol()

	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    addrProducer,
			Caller:      addr,
			EpochNumber: 1,
			BlockHeight: 1,
		},
	)
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.Initialize(ctx, ws, addr, big.NewInt(0), big.NewInt(10), big.NewInt(100)))
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	adminAddr, err := p.Admin(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, addr.Bytes(), adminAddr.Bytes())
	blockReward, err := p.BlockReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(10), blockReward)
	epochReward, err := p.EpochReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), epochReward)

	totalBalance, err := p.TotalBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), totalBalance)
	availableBalance, err := p.AvailableBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), availableBalance)

	// Create a test account with 1000 token
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(ws, addr.String(), big.NewInt(1000))
	require.NoError(t, err)
	require.NoError(t, stateDB.Commit(ws))

	test(t, ctx, stateDB, p)
}
