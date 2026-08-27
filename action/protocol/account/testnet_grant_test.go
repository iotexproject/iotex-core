// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// newGrantStateManager returns a state manager backed by an in-memory batch,
// the same shape the other tests in this package use.
func newGrantStateManager(t *testing.T) protocol.StateManager {
	ctrl := gomock.NewController(t)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()
	return sm
}

func grantCtx(g genesis.Genesis, height uint64) context.Context {
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(genesis.WithGenesisContext(ctx, g))
}

func TestCreatePreStatesGrant(t *testing.T) {
	r := require.New(t)

	var (
		fresh   = identityset.Address(10)
		funded  = identityset.Address(11)
		ungrant = identityset.Address(12)
		p       = NewProtocol(rewarding.DepositGas)
		sm      = newGrantStateManager(t)
	)

	g := genesis.TestDefault()
	g.Account.TestnetGrants = []genesis.TestnetGrant{{
		Height: 100,
		Recipients: []genesis.GrantRecipient{
			{Address: fresh.String(), Amount: "500"},
			{Address: funded.String(), Amount: "700"},
		},
	}}
	r.NoError(g.ValidateTestnetGrants())

	// `funded` already exists with a balance and a bumped nonce; the grant must
	// add to it rather than replace it, which is the whole reason this goes
	// through LoadOrCreateAccount instead of writing a fresh Account.
	pre, err := accountutil.LoadOrCreateAccount(sm, funded)
	r.NoError(err)
	r.NoError(pre.AddBalance(big.NewInt(42)))
	// two bumps, so the expected nonce below is one a fresh account could not
	// coincidentally have
	r.NoError(pre.SetPendingNonce(1))
	r.NoError(pre.SetPendingNonce(2))
	r.NoError(accountutil.StoreAccount(sm, funded, pre))

	// nothing happens at a height with no grant
	r.NoError(p.CreatePreStates(grantCtx(g, 99), sm))
	acct, err := accountutil.LoadAccount(sm, fresh)
	r.NoError(err)
	r.Equal("0", acct.Balance.String())

	r.NoError(p.CreatePreStates(grantCtx(g, 100), sm))

	acct, err = accountutil.LoadAccount(sm, fresh)
	r.NoError(err)
	r.Equal("500", acct.Balance.String())

	acct, err = accountutil.LoadAccount(sm, funded)
	r.NoError(err)
	r.Equal("742", acct.Balance.String())
	r.Equal(uint64(2), acct.PendingNonce())

	// an address not in the grant is untouched
	acct, err = accountutil.LoadAccount(sm, ungrant)
	r.NoError(err)
	r.Equal("0", acct.Balance.String())

	// the height after is a no-op again -- a grant fires once, at its height
	r.NoError(p.CreatePreStates(grantCtx(g, 101), sm))
	acct, err = accountutil.LoadAccount(sm, fresh)
	r.NoError(err)
	r.Equal("500", acct.Balance.String())
}

func TestCreatePreStatesNoGrants(t *testing.T) {
	r := require.New(t)
	p := NewProtocol(rewarding.DepositGas)
	sm := newGrantStateManager(t)
	g := genesis.TestDefault()
	for _, h := range []uint64{0, 1, 1000} {
		r.NoError(p.CreatePreStates(grantCtx(g, h), sm))
	}
}

// A malformed grant that somehow reached a running node -- ValidateTestnetGrants
// should have caught it at startup -- must fail the block rather than panic and
// take the node down.
func TestCreatePreStatesGrantMalformed(t *testing.T) {
	r := require.New(t)
	p := NewProtocol(rewarding.DepositGas)
	sm := newGrantStateManager(t)

	g := genesis.TestDefault()
	g.Account.TestnetGrants = []genesis.TestnetGrant{{
		Height:     100,
		Recipients: []genesis.GrantRecipient{{Address: "io1notavalidaddress", Amount: "1"}},
	}}
	r.Error(p.CreatePreStates(grantCtx(g, 100), sm))

	g.Account.TestnetGrants = []genesis.TestnetGrant{{
		Height:     100,
		Recipients: []genesis.GrantRecipient{{Address: identityset.Address(10).String(), Amount: "not-a-number"}},
	}}
	r.Error(p.CreatePreStates(grantCtx(g, 100), sm))
}

// The protocol has to actually be picked up as a PreStatesCreator, otherwise
// the hook above never runs and the grant silently does nothing.
func TestProtocolIsPreStatesCreator(t *testing.T) {
	var p interface{} = NewProtocol(rewarding.DepositGas)
	_, ok := p.(protocol.PreStatesCreator)
	require.True(t, ok)
}
