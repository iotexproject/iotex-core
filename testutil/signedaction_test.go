// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
)

var (
	addr1   = testaddress.Addrinfo["producer"].String()
	priKey1 = testaddress.Keyinfo["producer"].PriKey
	addr2   = testaddress.Addrinfo["alfa"].String()
)

func TestSignedTransfer(t *testing.T) {
	require := require.New(t)
	selp, err := SignedTransfer(addr2, priKey1, uint64(1), big.NewInt(2), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	tsf := selp.Action().(*action.Transfer)
	require.Equal(addr2, tsf.Recipient())
	require.Equal(uint64(1), tsf.Nonce())
	require.Equal(big.NewInt(2), tsf.Amount())
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), tsf.GasLimit())
	require.Equal(big.NewInt(10), tsf.GasPrice())
	require.NotNil(selp.Signature())
}

func TestSignedVote(t *testing.T) {
	require := require.New(t)
	selp, err := SignedVote(addr1, priKey1, uint64(1), uint64(100000), big.NewInt(10))
	require.NoError(err)

	vote := selp.Action().(*action.Vote)
	require.Equal(addr1, vote.Votee())
	require.Equal(uint64(1), vote.Nonce())
	require.Equal(uint64(100000), vote.GasLimit())
	require.Equal(big.NewInt(10), vote.GasPrice())
	require.NotNil(selp.Signature())
}

func TestSignedExecution(t *testing.T) {
	require := require.New(t)
	selp, err := SignedExecution(action.EmptyAddress, priKey1, uint64(1), big.NewInt(0), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(err)

	exec := selp.Action().(*action.Execution)
	require.Equal(action.EmptyAddress, exec.Contract())
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(big.NewInt(0), exec.Amount())
	require.Equal(uint64(100000), exec.GasLimit())
	require.Equal(big.NewInt(10), exec.GasPrice())
	require.Equal([]byte{}, exec.Data())
	require.NotNil(selp.Signature())
}
