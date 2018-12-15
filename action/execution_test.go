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
	executor, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	contract, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, chainid)
	require.NoError(err)
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex, err := NewExecution(executor.RawAddress, contract.RawAddress, 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetNonce(0).
		SetGasLimit(uint64(10)).
		SetDestinationAddress(contract.RawAddress).
		SetGasPrice(big.NewInt(10)).
		SetAction(ex).Build()

	w := AssembleSealedEnvelope(elp, executor.RawAddress, executor.PublicKey, []byte("lol"))
	require.Error(Verify(w))

	// sign the Execution
	selp, err := Sign(elp, executor.RawAddress, executor.PrivateKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}
