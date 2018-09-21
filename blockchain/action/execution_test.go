// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

func TestExecutionSignVerify(t *testing.T) {
	require := require.New(t)
	executor, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	contract, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)
	require.Error(Verify(ex, executor))

	// sign the Execution
	require.NoError(Sign(ex, executor))
	require.NotNil(ex)
	require.Equal(ex.Hash(), ex.Hash())

	// verify signature
	require.NoError(Verify(ex, executor))
	require.Error(Verify(ex, contract))
	require.NotNil(ex.Signature)
}

func TestExecutionSerializeDeserialize(t *testing.T) {
	require := require.New(t)
	executor, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	contract, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	data, err := hex.DecodeString("60652403")
	require.NoError(err)

	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(123), uint64(1234), big.NewInt(10), data)
	require.NoError(err)
	require.NotNil(ex)

	s, err := ex.Serialize()
	require.NoError(err)

	newex := &Execution{}
	err = newex.Deserialize(s)
	require.NoError(err)

	require.Equal(uint64(0), newex.Nonce())
	require.Equal(uint64(123), newex.amount.Uint64())
	require.Equal(executor.RawAddress, newex.Executor())
	require.Equal(contract.RawAddress, newex.Contract())

	require.Equal(ex.Hash(), newex.Hash())
	require.Equal(ex.TotalSize(), newex.TotalSize())
}

func TestExecutionToJSONFromJSON(t *testing.T) {
	require := require.New(t)
	executor, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	contract, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	data, err := hex.DecodeString("60652403")
	require.NoError(err)

	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(123), uint64(1234), big.NewInt(10), data)
	require.NoError(err)
	require.NotNil(ex)

	expex, err := ex.ToJSON()
	require.NoError(err)
	require.NotNil(expex)

	newex, err := NewExecutionFromJSON(expex)
	require.NoError(err)
	require.NotNil(newex)

	require.Equal(uint64(0), newex.Nonce())
	require.Equal(uint64(123), newex.amount.Uint64())
	require.Equal(executor.RawAddress, newex.Executor())
	require.Equal(contract.RawAddress, newex.Contract())

	require.Equal(ex.Hash(), newex.Hash())
	require.Equal(ex.TotalSize(), newex.TotalSize())
}
