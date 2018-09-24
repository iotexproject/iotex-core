// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"github.com/iotexproject/iotex-core/blockchain/action"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	pubkeyA = "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikeyA = "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
	pubkeyB = "881504d84a0659e14dcba59f24a98e71cda55b139615342668840c64678f1514941bbd053c7492fb9b719e6050cfa972efa491b79e11a1713824dda5f638fc0d9fa1b68be3c0f905"
	prikeyB = "b89c1ec0fb5b192c8bb8f6fcf9a871e4a67ef462f40d2b8ff426da1d1eaedd9696dc9d00"
)

var (
	addr1 = ConstructAddress(pubkeyA, prikeyA)
	addr2 = ConstructAddress(pubkeyB, prikeyB)
)

func TestSignedTransfer(t *testing.T) {
	require := require.New(t)
	tsf, err := SignedTransfer(addr1, addr2, uint64(1), big.NewInt(2),
		[]byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.Equal(addr1.RawAddress, tsf.Sender())
	require.Equal(addr2.RawAddress, tsf.Recipient())
	require.Equal(uint64(1), tsf.Nonce())
	require.Equal(big.NewInt(2), tsf.Amount())
	require.Equal([]byte{}, tsf.Payload())
	require.Equal(uint64(100000), tsf.GasLimit())
	require.Equal(big.NewInt(10), tsf.GasPrice())
	require.NotNil(tsf.Signature())
}

func TestSignedVote(t *testing.T) {
	require := require.New(t)
	vote, err := SignedVote(addr1, addr1, uint64(1), uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.Equal(addr1.RawAddress, vote.Voter())
	require.Equal(addr1.RawAddress, vote.Votee())
	require.Equal(uint64(1), vote.Nonce())
	require.Equal(uint64(100000), vote.GasLimit())
	require.Equal(big.NewInt(10), vote.GasPrice())
	require.NotNil(vote.Signature())
}

func TestSignedExecution(t *testing.T) {
	require := require.New(t)
	exec, err := SignedExecution(addr1, action.EmptyAddress, uint64(1), big.NewInt(0),
		uint64(100000), big.NewInt(10), []byte{})
	require.NoError(err)
	require.Equal(addr1.RawAddress, exec.Executor())
	require.Equal(action.EmptyAddress, exec.Contract())
	require.Equal(uint64(1), exec.Nonce())
	require.Equal(big.NewInt(0), exec.Amount())
	require.Equal(uint64(100000), exec.GasLimit())
	require.Equal(big.NewInt(10), exec.GasPrice())
	require.Equal([]byte{}, exec.Data())
	require.NotNil(exec.Signature())
}
