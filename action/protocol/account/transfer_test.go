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

	accountAlfa := state.Account{
		Balance: big.NewInt(50005),
		Votee:   testaddress.Addrinfo["charlie"].String(),
	}
	accountBravo := state.Account{
		Votee: testaddress.Addrinfo["delta"].String(),
	}
	accountCharlie := state.Account{
		VotingWeight: big.NewInt(5),
	}
	pubKeyAlfa := hash.BytesToHash160(testaddress.Addrinfo["alfa"].Bytes())
	pubKeyBravo := hash.BytesToHash160(testaddress.Addrinfo["bravo"].Bytes())
	pubKeyCharlie := hash.BytesToHash160(testaddress.Addrinfo["charlie"].Bytes())
	pubKeyDelta := hash.BytesToHash160(testaddress.Addrinfo["delta"].Bytes())

	require.NoError(ws.PutState(pubKeyAlfa, &accountAlfa))
	require.NoError(ws.PutState(pubKeyBravo, &accountBravo))
	require.NoError(ws.PutState(pubKeyCharlie, &accountCharlie))

	transfer, err := action.NewTransfer(
		uint64(1),
		big.NewInt(2),
		testaddress.Addrinfo["bravo"].String(),
		[]byte{},
		uint64(10000),
		big.NewInt(1),
	)
	require.NoError(err)
	gas, err := transfer.IntrinsicGas()
	require.NoError(err)
	ctx = protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer:     testaddress.Addrinfo["producer"],
			Caller:       testaddress.Addrinfo["alfa"],
			GasLimit:     testutil.TestGasLimit,
			IntrinsicGas: gas,
		})
	receipt, err := p.Handle(ctx, transfer, ws)
	require.NoError(err)
	require.Equal(action.SuccessReceiptStatus, receipt.Status)
	require.NoError(sf.Commit(ws))

	var acct state.Account
	require.NoError(sf.State(pubKeyAlfa, &acct))
	require.Equal("40003", acct.Balance.String())
	require.Equal(uint64(1), acct.Nonce)
	require.NoError(sf.State(pubKeyBravo, &acct))
	require.Equal("2", acct.Balance.String())
	require.NoError(sf.State(pubKeyCharlie, &acct))
	require.Equal("3", acct.VotingWeight.String())
	require.NoError(sf.State(pubKeyDelta, &acct))
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
		big.NewInt(2),
	)
	require.NoError(err)
	// Assume that the gas of this transfer is the same as previous one
	receipt, err = p.Handle(ctx, transfer, ws)
	require.NoError(err)
	require.Equal(action.FailureReceiptStatus, receipt.Status)
	require.NoError(sf.Commit(ws))
	require.NoError(sf.State(pubKeyAlfa, &acct))
	require.Equal(uint64(2), acct.Nonce)
	require.Equal("20003", acct.Balance.String())
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
