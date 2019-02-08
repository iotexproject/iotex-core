// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestProtocol_Handle(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	ctx := context.Background()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)

	p := NewProtocol(nil)

	// Create three accounts
	addr1 := testaddress.Addrinfo["alfa"].String()
	addr2 := testaddress.Addrinfo["bravo"].String()
	addr3 := testaddress.Addrinfo["charlie"].String()
	k1 := testaddress.Keyinfo["alfa"]
	k2 := testaddress.Keyinfo["bravo"]
	k3 := testaddress.Keyinfo["charlie"]
	pkHash1 := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
	pkHash2 := byteutil.BytesTo20B(testaddress.Addrinfo["bravo"].Bytes())
	pkHash3 := byteutil.BytesTo20B(testaddress.Addrinfo["charlie"].Bytes())

	_, err = account.LoadOrCreateAccount(ws, addr1, big.NewInt(100))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, addr2, big.NewInt(100))
	require.NoError(err)
	_, err = account.LoadOrCreateAccount(ws, addr3, big.NewInt(100))
	require.NoError(err)

	checkSelfNomination := func(address string, account *state.Account) {
		require.Equal(uint64(1), account.Nonce)
		require.True(account.IsCandidate)
		require.Equal(address, account.Votee)
		require.Equal("100", account.VotingWeight.String())
	}

	vote1, err := testutil.SignedVote(addr1, addr1, k1.PriKey, 1, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["alfa"],
		},
	)
	_, err = p.Handle(ctx, vote1.Action(), ws)
	require.NoError(err)
	account1, _ := account.LoadAccount(ws, pkHash1)
	checkSelfNomination(addr1, account1)

	vote2, err := testutil.SignedVote(addr2, addr2, k2.PriKey, 1, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["bravo"],
		},
	)
	_, err = p.Handle(ctx, vote2.Action(), ws)
	require.NoError(err)
	account2, _ := account.LoadAccount(ws, pkHash2)
	checkSelfNomination(addr2, account2)

	vote3, err := testutil.SignedVote(addr3, addr3, k3.PriKey, 1, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["charlie"],
		},
	)
	_, err = p.Handle(ctx, vote3.Action(), ws)
	require.NoError(err)
	account3, _ := account.LoadAccount(ws, pkHash3)
	checkSelfNomination(addr3, account3)

	unvote1, err := testutil.SignedVote(addr1, "", k1.PriKey, 2, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["alfa"],
		},
	)
	_, err = p.Handle(ctx, unvote1.Action(), ws)
	require.NoError(err)
	account1, _ = account.LoadAccount(ws, pkHash1)
	require.Equal(uint64(2), account1.Nonce)
	require.False(account1.IsCandidate)
	require.Equal("", account1.Votee)
	require.Equal("0", account1.VotingWeight.String())

	vote4, err := testutil.SignedVote(addr2, addr3, k2.PriKey, 2, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["bravo"],
		},
	)
	_, err = p.Handle(ctx, vote4.Action(), ws)
	require.NoError(err)
	account2, _ = account.LoadAccount(ws, pkHash2)
	account3, _ = account.LoadAccount(ws, pkHash3)
	require.Equal(uint64(2), account2.Nonce)
	require.True(account2.IsCandidate)
	require.Equal(addr3, account2.Votee)
	require.Equal("0", account2.VotingWeight.String())
	require.Equal("200", account3.VotingWeight.String())

	unvote2, err := testutil.SignedVote(addr2, "", k2.PriKey, 3, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
			Caller:          testaddress.Addrinfo["bravo"],
		},
	)
	_, err = p.Handle(ctx, unvote2.Action(), ws)
	require.NoError(err)
	account2, _ = account.LoadAccount(ws, pkHash2)
	account3, _ = account.LoadAccount(ws, pkHash3)
	require.Equal(uint64(3), account2.Nonce)
	require.False(account2.IsCandidate)
	require.Equal("", account2.Votee)
	require.Equal("0", account2.VotingWeight.String())
	require.Equal("100", account3.VotingWeight.String())

	canidateMap, err := candidatesutil.GetMostRecentCandidateMap(ws)
	require.NoError(err)
	require.Equal(1, len(canidateMap))
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(
		testaddress.Addrinfo["producer"].String(),
		big.NewInt(0),
	)
	_, err = bc.CreateState(
		testaddress.Addrinfo["alfa"].String(),
		big.NewInt(0),
	)
	require.NoError(err)
	p := NewProtocol(bc)

	// Caes I: Oversized data
	var dst string
	for i := 0; i < 10000; i++ {
		dst += "a"
	}
	vote, err := action.NewVote(1, dst, uint64(100000), big.NewInt(0))
	require.NoError(err)
	ctx := protocol.WithValidateActionsCtx(context.Background(), protocol.ValidateActionsCtx{
		Caller: testaddress.Addrinfo["producer"],
	})
	err = p.Validate(ctx, vote)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Invalid votee address
	vote, err = action.NewVote(1, "123", uint64(100000),
		big.NewInt(0))
	require.NoError(err)
	err = p.Validate(ctx, vote)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating votee's address"))
	// Case III: Votee is not a candidate
	vote2, err := action.NewVote(1, testaddress.Addrinfo["alfa"].String(), uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = p.Validate(ctx, vote2)
	require.Equal(action.ErrVotee, errors.Cause(err))
}
