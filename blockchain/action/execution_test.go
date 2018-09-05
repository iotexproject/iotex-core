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
	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(10), 10, 10, data)
	require.NoError(err)
	require.Nil(ex.Signature)
	require.NotNil(ex.Verify(executor))

	// sign the Execution
	sex, err := ex.Sign(executor)
	require.NoError(err)
	require.NotNil(sex)
	require.Equal(ex.Hash(), sex.Hash())

	ex.ExecutorPubKey = executor.PublicKey
	require.Equal(ex.Hash(), sex.Hash())

	// verify signature
	require.NoError(sex.Verify(executor))
	require.NotNil(sex.Verify(contract))
	require.NotNil(ex.Signature)
	require.NoError(ex.Verify(executor))
}

func TestExecutionSerializeDeserialize(t *testing.T) {
	require := require.New(t)
	executor, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	contract, err := iotxaddress.NewAddress(true, chainid)
	require.NoError(err)
	data, err := hex.DecodeString("60652403")
	require.NoError(err)

	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(123), 1234, 10, data)
	require.NoError(err)
	require.NotNil(ex)

	s, err := ex.Serialize()
	require.NoError(err)

	newex := &Execution{}
	err = newex.Deserialize(s)
	require.NoError(err)

	require.Equal(uint64(0), newex.Nonce)
	require.Equal(uint64(123), newex.Amount.Uint64())
	require.Equal(executor.RawAddress, newex.Executor)
	require.Equal(contract.RawAddress, newex.Contract)

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

	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(123), 1234, 10, data)
	require.NoError(err)
	require.NotNil(ex)

	expex, err := ex.ToJSON()
	require.NoError(err)
	require.NotNil(expex)

	newex, err := NewExecutionFromJSON(expex)
	require.NoError(err)
	require.NotNil(newex)

	require.Equal(uint64(0), newex.Nonce)
	require.Equal(uint64(123), newex.Amount.Uint64())
	require.Equal(executor.RawAddress, newex.Executor)
	require.Equal(contract.RawAddress, newex.Contract)

	require.Equal(ex.Hash(), newex.Hash())
	require.Equal(ex.TotalSize(), newex.TotalSize())
}
