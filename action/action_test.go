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

func TestActionProtoAndVerify(t *testing.T) {
	require := require.New(t)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v, err := NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)
	t.Run("no error", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)

		require.NoError(Verify(selp))

		nselp := &SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))

		selpHash, err := selp.Hash()
		require.NoError(err)
		nselpHash, err := nselp.Hash()
		require.NoError(err)
		require.Equal(selpHash, nselpHash)
	})
	t.Run("empty public key", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)

		selp.srcPubkey = nil

		require.EqualError(Verify(selp), "empty public key")
	})
	t.Run("gas limit too low", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(1000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)

		require.Equal(ErrInsufficientBalanceForGas, errors.Cause(Verify(selp)))
	})
	t.Run("invalid signature", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()

		selp, err := Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		selp.signature = []byte("invalid signature")

		require.Contains(Verify(selp).Error(), "failed to verify action hash")
	})
}
