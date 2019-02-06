// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestProtocol_HandleTransfer(t *testing.T) {
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

	p := NewProtocol()

	account1 := state.Account{
		Balance: big.NewInt(5),
		Votee:   testaddress.Addrinfo["charlie"].Bech32(),
	}
	account2 := state.Account{
		Votee: testaddress.Addrinfo["delta"].Bech32(),
	}
	account3 := state.Account{
		VotingWeight: big.NewInt(5),
	}
	pubKeyHash1 := testaddress.Addrinfo["alfa"].PublicKeyHash()
	pubKeyHash2 := testaddress.Addrinfo["bravo"].PublicKeyHash()
	pubKeyHash3 := testaddress.Addrinfo["charlie"].PublicKeyHash()
	pubKeyHash4 := testaddress.Addrinfo["delta"].PublicKeyHash()

	require.NoError(ws.PutState(pubKeyHash1, &account1))
	require.NoError(ws.PutState(pubKeyHash2, &account2))
	require.NoError(ws.PutState(pubKeyHash3, &account3))

	transfer, err := action.NewTransfer(uint64(1), big.NewInt(2), testaddress.Addrinfo["alfa"].Bech32(),
		testaddress.Addrinfo["bravo"].Bech32(), []byte{}, uint64(10000), big.NewInt(0))
	require.NoError(err)

	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EnableGasCharge: false,
		})
	_, err = p.Handle(ctx, transfer, ws)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	var s1 state.Account
	err = sf.State(pubKeyHash1, &s1)
	require.NoError(err)
	var s2 state.Account
	err = sf.State(pubKeyHash2, &s2)
	require.NoError(err)
	var s3 state.Account
	err = sf.State(pubKeyHash3, &s3)
	require.NoError(err)
	var s4 state.Account
	err = sf.State(pubKeyHash4, &s4)
	require.NoError(err)

	require.Equal("3", s1.Balance.String())
	require.Equal(uint64(1), s1.Nonce)
	require.Equal("2", s2.Balance.String())
	require.Equal("3", s3.VotingWeight.String())
	require.Equal("2", s4.VotingWeight.String())
}

func TestProtocol_ValidateTransfer(t *testing.T) {
	require := require.New(t)
	protocol := NewProtocol()
	// Case I: Oversized data
	tmpPayload := [32769]byte{}
	payload := tmpPayload[:]
	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), "1", "2", payload, uint64(0),
		big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Negative amount
	tsf, err = action.NewTransfer(uint64(1), big.NewInt(-100), "1", "2", nil,
		uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case III: Invalid recipient address
	tsf, err = action.NewTransfer(
		1,
		big.NewInt(1),
		testaddress.Addrinfo["producer"].Bech32(),
		testaddress.Addrinfo["alfa"].Bech32()+"aaa",
		nil,
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
}
