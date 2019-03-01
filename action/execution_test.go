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

	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestExecutionSignVerify(t *testing.T) {
	require := require.New(t)
	contractAddr := testaddress.Addrinfo["alfa"]
	executorKey := testaddress.Keyinfo["producer"]
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex, err := NewExecution(contractAddr.String(), 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetNonce(0).
		SetGasLimit(uint64(10)).
		SetGasPrice(big.NewInt(10)).
		SetAction(ex).Build()

	w := AssembleSealedEnvelope(elp, executorKey.PubKey, []byte("lol"))
	require.Error(Verify(w))

	// sign the Execution
	selp, err := Sign(elp, executorKey.PriKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}
