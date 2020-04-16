// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"strings"
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
	ex, err := NewExecution(contractAddr.String(), big.NewInt(10), data)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp, err := bd.SetNonce(0).
		SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(ex).Build()
	require.NoError(err)

	w := AssembleSealedEnvelope(elp, executorKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	// sign the Execution
	selp, err := Sign(elp, executorKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
	t.Run("Negative amount", func(t *testing.T) {
		ex, err := NewExecution("2", big.NewInt(-100), []byte{})
		require.NoError(err)
		require.Equal(ErrInvalidAmount, errors.Cause(ex.SanityCheck()))
	})

	t.Run("Invalid contract address", func(t *testing.T) {
		ex, err := NewExecution(
			identityset.Address(29).String()+"bbb",
			big.NewInt(0),
			[]byte{},
		)
		require.NoError(err)
		require.True(strings.Contains(ex.SanityCheck().Error(), "error when validating contract's address"))
	})
}
