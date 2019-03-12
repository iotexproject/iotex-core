// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestLoadOrCreateAccountState(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	sf, err := factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(db.NewMemKVStore()))
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	addrv1 := testaddress.Addrinfo["producer"]
	s, err := accountutil.LoadAccount(ws, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(s.Balance, state.EmptyAccount().Balance)
	require.Equal(s.VotingWeight, state.EmptyAccount().VotingWeight)
	s, err = accountutil.LoadOrCreateAccount(ws, addrv1.String(), big.NewInt(5))
	require.NoError(err)
	s, err = accountutil.LoadAccount(ws, hash.BytesToHash160(addrv1.Bytes()))
	require.NoError(err)
	require.Equal(uint64(0x0), s.Nonce)
	require.Equal("5", s.Balance.String())

	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))
	ss, err := sf.AccountState(addrv1.String())
	require.Nil(err)
	require.Equal(uint64(0x0), ss.Nonce)
	require.Equal("5", ss.Balance.String())
}

func TestProtocol_Initialize(t *testing.T) {
	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	p := NewProtocol()

	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(
		t,
		p.Initialize(
			protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
				BlockHeight: 0,
			}),
			ws,
			[]address.Address{
				identityset.Address(0),
				identityset.Address(1),
			},
			[]*big.Int{
				big.NewInt(100),
				big.NewInt(200),
			},
		),
	)
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	acc0, err := accountutil.LoadOrCreateAccount(ws, identityset.Address(0).String(), big.NewInt(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), acc0.Balance)
	acc1, err := accountutil.LoadOrCreateAccount(ws, identityset.Address(1).String(), big.NewInt(0))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(200), acc1.Balance)
}
