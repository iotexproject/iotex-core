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

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocols/account"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestProtocol_Handle(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	ctx := context.Background()
	sf, err := state.NewFactory(cfg, state.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)

	mbc := mock_blockchain.NewMockBlockchain(ctrl)
	protocol := NewProtocol(mbc)

	// Create three accounts
	addr1 := testaddress.Addrinfo["alfa"].RawAddress
	addr2 := testaddress.Addrinfo["bravo"].RawAddress
	addr3 := testaddress.Addrinfo["charlie"].RawAddress
	pkHash1, _ := iotxaddress.AddressToPKHash(addr1)
	pkHash2, _ := iotxaddress.AddressToPKHash(addr2)
	pkHash3, _ := iotxaddress.AddressToPKHash(addr3)

	_, err = account.LoadOrCreateAccountState(ws, addr1, big.NewInt(100))
	require.NoError(err)
	_, err = account.LoadOrCreateAccountState(ws, addr2, big.NewInt(100))
	require.NoError(err)
	_, err = account.LoadOrCreateAccountState(ws, addr3, big.NewInt(100))
	require.NoError(err)

	checkSelfNomination := func(address string, account *state.Account) {
		require.Equal(uint64(1), account.Nonce)
		require.True(account.IsCandidate)
		require.Equal(address, account.Votee)
		require.Equal("100", account.VotingWeight.String())
	}

	vote1, err := action.NewVote(1, addr1, addr1, uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, vote1, ws)
	require.NoError(err)
	account1, _ := account.LoadAccountState(ws, pkHash1)
	checkSelfNomination(addr1, account1)

	vote2, err := action.NewVote(1, addr2, addr2, uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, vote2, ws)
	require.NoError(err)
	account2, _ := account.LoadAccountState(ws, pkHash2)
	checkSelfNomination(addr2, account2)

	vote3, err := action.NewVote(1, addr3, addr3, uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, vote3, ws)
	require.NoError(err)
	account3, _ := account.LoadAccountState(ws, pkHash3)
	checkSelfNomination(addr3, account3)

	unvote1, err := action.NewVote(2, addr1, "", uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, unvote1, ws)
	require.NoError(err)
	account1, _ = account.LoadAccountState(ws, pkHash1)
	require.Equal(uint64(2), account1.Nonce)
	require.False(account1.IsCandidate)
	require.Equal("", account1.Votee)
	require.Equal("0", account1.VotingWeight.String())

	vote4, err := action.NewVote(2, addr2, addr3, uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, vote4, ws)
	require.NoError(err)
	account2, _ = account.LoadAccountState(ws, pkHash2)
	account3, _ = account.LoadAccountState(ws, pkHash3)
	require.Equal(uint64(2), account2.Nonce)
	require.True(account2.IsCandidate)
	require.Equal(addr3, account2.Votee)
	require.Equal("0", account2.VotingWeight.String())
	require.Equal("200", account3.VotingWeight.String())

	unvote2, err := action.NewVote(3, addr2, "", uint64(100000), big.NewInt(0))
	require.NoError(err)
	_, err = protocol.Handle(ctx, unvote2, ws)
	require.NoError(err)
	account2, _ = account.LoadAccountState(ws, pkHash2)
	account3, _ = account.LoadAccountState(ws, pkHash3)
	require.Equal(uint64(3), account2.Nonce)
	require.False(account2.IsCandidate)
	require.Equal("", account2.Votee)
	require.Equal("0", account2.VotingWeight.String())
	require.Equal("100", account3.VotingWeight.String())

	canidateMap, err := protocol.getCandidateMap(uint64(0), ws)
	require.NoError(err)
	require.Equal(1, len(canidateMap))
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	bc := blockchain.NewBlockchain(config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(bc.Start(context.Background()))
	_, err := bc.CreateState(
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0),
	)
	_, err = bc.CreateState(
		testaddress.Addrinfo["alfa"].RawAddress,
		big.NewInt(0),
	)
	require.NoError(err)
	protocol := NewProtocol(bc)

	// Caes I: Oversized data
	vote, err := action.NewVote(1, "src", "dst", uint64(100000), big.NewInt(0))
	require.NoError(err)
	invalidSig := [1000]byte{}
	vote.SetSignature(invalidSig[:])
	err = protocol.Validate(context.Background(), vote)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Invalid address
	vote, err = action.NewVote(1, testaddress.Addrinfo["producer"].RawAddress, "123", uint64(100000),
		big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), vote)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating votee's address"))
	// Case III: Votee is not a candidate
	vote2, err := action.NewVote(1, testaddress.Addrinfo["producer"].RawAddress,
		testaddress.Addrinfo["alfa"].RawAddress, uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), vote2)
	require.Equal(action.ErrVotee, errors.Cause(err))
}
