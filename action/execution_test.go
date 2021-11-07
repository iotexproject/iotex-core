// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestExecutionSignVerify(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(28)
	executorKey := identityset.PrivateKey(27)
	data, err := hex.DecodeString("")
	require.NoError(err)
	ex, err := NewExecution(contractAddr.String(), 2, big.NewInt(10), uint64(100000), big.NewInt(10), data)
	require.NoError(err)
	require.Nil(ex.srcPubkey)

	bd := &EnvelopeBuilder{}
	eb := bd.SetNonce(ex.nonce).
		SetGasLimit(ex.gasLimit).
		SetGasPrice(ex.gasPrice).
		SetAction(ex).Build()
	elp, ok := eb.(*envelope)
	require.True(ok)

	w := AssembleSealedEnvelope(elp, executorKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))
	ex2, ok := w.Envelope.Action().(*Execution)
	require.True(ok)
	require.Equal(ex, ex2)
	require.NotNil(ex.srcPubkey)

	// sign the Execution
	selp, err := Sign(elp, executorKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
	t.Run("Negative amount", func(t *testing.T) {
		ex, err := NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(0), []byte{})
		require.NoError(err)
		require.Equal(ErrInvalidAmount, errors.Cause(ex.SanityCheck()))
	})

	t.Run("Invalid contract address", func(t *testing.T) {
		ex, err := NewExecution(
			identityset.Address(29).String()+"bbb",
			uint64(1),
			big.NewInt(0),
			uint64(0),
			big.NewInt(0),
			[]byte{},
		)
		require.NoError(err)
		require.Contains(ex.SanityCheck().Error(), "error when validating contract's address")
	})

	t.Run("Negative gas price", func(t *testing.T) {
		ex, err := NewExecution(identityset.Address(29).String(), uint64(1), big.NewInt(100), uint64(0), big.NewInt(-1), []byte{})
		require.NoError(err)
		require.Equal(ErrGasPrice, errors.Cause(ex.SanityCheck()))
	})
}
