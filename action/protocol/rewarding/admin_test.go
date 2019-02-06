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

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state/factory"
)

func TestProtocol_Admin(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, stateDB factory.Factory, p *Protocol) {
		// Update block reward
		ws, err := stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.SetBlockReward(ctx, ws, big.NewInt(20)))
		stateDB.Commit(ws)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		blockReward, err := p.BlockReward(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(20), blockReward)

		// Set block reward again will fail because caller is not admin
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		skNoAuth, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHashNoAuth := keypair.HashPubKey(&skNoAuth.PublicKey)
		addrNoAuth := address.New(pkHashNoAuth[:])
		require.Error(t, p.SetBlockReward(
			protocol.WithRunActionsCtx(
				context.Background(),
				protocol.RunActionsCtx{
					Caller: addrNoAuth,
				},
			),
			ws,
			big.NewInt(30),
		))

		// Update epoch reward
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.NoError(t, p.SetEpochReward(ctx, ws, big.NewInt(200)))
		stateDB.Commit(ws)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		epochReward, err := p.EpochReward(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(200), epochReward)

		// Set epoch reward again will fail because caller is not admin
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		require.Error(t, p.SetEpochReward(
			protocol.WithRunActionsCtx(
				context.Background(),
				protocol.RunActionsCtx{
					Caller: addrNoAuth,
				},
			),
			ws,
			big.NewInt(300),
		))

		// Update admin
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		skNew, err := crypto.GenerateKey()
		require.NoError(t, err)
		pkHashNew := keypair.HashPubKey(&skNew.PublicKey)
		addrNew := address.New(pkHashNew[:])
		require.NoError(t, p.SetAdmin(ctx, ws, addrNew))
		stateDB.Commit(ws)

		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		adminAddr, err := p.Admin(ctx, ws)
		require.NoError(t, err)
		assert.Equal(t, addrNew.Bytes(), adminAddr.Bytes())

		// Update admin again will fail because addr is no longer the admin
		ws, err = stateDB.NewWorkingSet()
		require.NoError(t, err)
		skNew, err = crypto.GenerateKey()
		require.NoError(t, err)
		pkHashNew = keypair.HashPubKey(&skNew.PublicKey)
		addrNew = address.New(pkHashNew[:])
		require.Error(t, p.SetAdmin(ctx, ws, addrNew))
	})

}
