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

	"github.com/iotexproject/iotex-core/testutil"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
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
		Votee:   testaddress.Addrinfo["charlie"].String(),
	}
	account2 := state.Account{
		Votee: testaddress.Addrinfo["delta"].String(),
	}
	account3 := state.Account{
		VotingWeight: big.NewInt(5),
	}
	pubKeyHash1 := hash.BytesToHash160(testaddress.Addrinfo["alfa"].Bytes())
	pubKeyHash2 := hash.BytesToHash160(testaddress.Addrinfo["bravo"].Bytes())
	pubKeyHash3 := hash.BytesToHash160(testaddress.Addrinfo["charlie"].Bytes())
	pubKeyHash4 := hash.BytesToHash160(testaddress.Addrinfo["delta"].Bytes())

	require.NoError(ws.PutState(pubKeyHash1, &account1))
	require.NoError(ws.PutState(pubKeyHash2, &account2))
	require.NoError(ws.PutState(pubKeyHash3, &account3))

	transfer, err := action.NewTransfer(
		uint64(1),
		big.NewInt(2),
		testaddress.Addrinfo["bravo"].String(),
		[]byte{},
		uint64(10000),
		big.NewInt(0),
	)
	require.NoError(err)

	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			Caller:   testaddress.Addrinfo["alfa"],
			GasLimit: testutil.TestGasLimit,
		})
	receipt, err := p.Handle(ctx, transfer, ws)
	require.NoError(err)
	require.Equal(action.SuccessReceiptStatus, receipt.Status)
	require.NoError(sf.Commit(ws))

	var acct state.Account
	require.NoError(sf.State(pubKeyHash1, &acct))
	require.Equal("3", acct.Balance.String())
	require.Equal(uint64(1), acct.Nonce)
	require.NoError(sf.State(pubKeyHash2, &acct))
	require.Equal("2", acct.Balance.String())
	require.NoError(sf.State(pubKeyHash3, &acct))
	require.Equal("3", acct.VotingWeight.String())
	require.NoError(sf.State(pubKeyHash4, &acct))
	require.Equal("2", acct.VotingWeight.String())

	contractAcct := state.Account{
		CodeHash: []byte("codeHash"),
	}
	contractAddr := hash.BytesToHash160(testaddress.Addrinfo["echo"].Bytes())
	require.NoError(ws.PutState(contractAddr, &contractAcct))
	transfer, err = action.NewTransfer(
		uint64(2),
		big.NewInt(3),
		testaddress.Addrinfo["echo"].String(),
		[]byte{},
		uint64(10000),
		big.NewInt(0),
	)
	require.NoError(err)
	receipt, err = p.Handle(ctx, transfer, ws)
	require.NoError(err)
	require.Equal(action.FailureReceiptStatus, receipt.Status)
}

func TestProtocol_ValidateTransfer(t *testing.T) {
	require := require.New(t)
	protocol := NewProtocol()
	// Case I: Oversized data
	tmpPayload := [32769]byte{}
	payload := tmpPayload[:]
	tsf, err := action.NewTransfer(uint64(1), big.NewInt(1), "2", payload, uint64(0),
		big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Negative amount
	tsf, err = action.NewTransfer(uint64(1), big.NewInt(-100), "2", nil,
		uint64(100000), big.NewInt(0))
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case III: Invalid recipient address
	tsf, err = action.NewTransfer(
		1,
		big.NewInt(1),
		testaddress.Addrinfo["alfa"].String()+"aaa",
		nil,
		uint64(100000),
		big.NewInt(0),
	)
	require.NoError(err)
	err = protocol.Validate(context.Background(), tsf)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating recipient's address"))
}
