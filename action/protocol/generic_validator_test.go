// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestActionProto(t *testing.T) {
	require := require.New(t)
	caller, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	ctx := ValidateActionsCtx{1, "io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", caller}
	c := WithValidateActionsCtx(context.Background(), ctx)
	cm := &MockChainManager{}
	valid := NewGenericValidator(cm)
	data, err := hex.DecodeString("")
	require.NoError(err)
	// Case I: Normal
	{
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		require.NoError(valid.Validate(c, nselp))
	}
	// Case II: GasLimit lower
	{
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(10)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(c, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "insufficient gas"))
	}
	// Case III: Call cm Nonce err
	{
		caller, err := address.FromString("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6")
		require.NoError(err)
		ctx := ValidateActionsCtx{1, "io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", caller}
		c := WithValidateActionsCtx(context.Background(), ctx)
		v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(c, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "invalid nonce value of account"))
	}
	// Case IV: Call Nonce err
	{
		v, err := action.NewExecution("", 1, big.NewInt(10), uint64(10), big.NewInt(10), data)
		require.NoError(err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasPrice(big.NewInt(10)).
			SetNonce(1).
			SetGasLimit(uint64(100000)).
			SetAction(v).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(28))
		require.NoError(err)
		nselp := action.SealedEnvelope{}
		require.NoError(nselp.LoadProto(selp.Proto()))
		err = valid.Validate(c, nselp)
		require.Error(err)
		require.True(strings.Contains(err.Error(), "nonce is too low"))
	}
}
