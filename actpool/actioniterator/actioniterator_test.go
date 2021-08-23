// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actioniterator

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestActionIterator(t *testing.T) {
	require := require.New(t)

	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29)
	priKeyB := identityset.PrivateKey(29)
	c := identityset.Address(30)
	priKeyC := identityset.PrivateKey(30)
	accMap := make(map[string][]action.SealedEnvelope)
	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), b.String(), nil, uint64(0), big.NewInt(13))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetGasPrice(big.NewInt(13)).
		SetAction(tsf1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tsf2, err := action.NewTransfer(uint64(2), big.NewInt(100), "2", nil, uint64(0), big.NewInt(30))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(30)).
		SetAction(tsf2).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	accMap[a.String()] = []action.SealedEnvelope{selp1, selp2}

	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(100), c.String(), nil, uint64(0), big.NewInt(15))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(15)).
		SetAction(tsf3).Build()
	selp3, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	tsf4, err := action.NewTransfer(uint64(2), big.NewInt(100), "3", nil, uint64(0), big.NewInt(10))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf4).Build()
	selp4, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	tsf5, err := action.NewTransfer(uint64(3), big.NewInt(100), a.String(), nil, uint64(0), big.NewInt(2))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasPrice(big.NewInt(20)).
		SetAction(tsf5).Build()
	selp5, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	accMap[b.String()] = []action.SealedEnvelope{selp3, selp4, selp5}

	tsf6, err := action.NewTransfer(uint64(1), big.NewInt(100), "1", nil, uint64(0), big.NewInt(5))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(5)).
		SetAction(tsf6).Build()
	selp6, err := action.Sign(elp, priKeyC)
	require.NoError(err)

	accMap[c.String()] = []action.SealedEnvelope{selp6}

	ai := NewActionIterator(accMap)
	appliedActionList := make([]action.SealedEnvelope, 0)
	for {
		bestAction, ok := ai.Next()
		if !ok {
			break
		}
		appliedActionList = append(appliedActionList, bestAction)
	}
	require.Equal(appliedActionList, []action.SealedEnvelope{selp3, selp1, selp2, selp4, selp5, selp6})
}

func BenchmarkLooping(b *testing.B) {
	accMap := make(map[string][]action.SealedEnvelope)
	for i := 0; i < b.N; i++ {
		priKey, err := crypto.HexStringToPrivateKey(fmt.Sprintf("1%063x", i))
		require.NoError(b, err)
		addr := priKey.PublicKey().Address()
		require.NotNil(b, addr)
		tsf, err := action.NewTransfer(uint64(1), big.NewInt(100), "1", nil, uint64(100000), big.NewInt(int64(i)))
		require.NoError(b, err)
		bd := &action.EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetGasPrice(big.NewInt(5)).
			SetAction(tsf).Build()
		selp, err := action.Sign(elp, priKey)
		require.NoError(b, err)
		accMap[addr.String()] = []action.SealedEnvelope{selp}
	}
	ai := NewActionIterator(accMap)
	b.ResetTimer()
	for {
		act, ok := ai.Next()
		if !ok {
			break
		}
		if act.GasLimit() < 30 {
			ai.PopAccount()
		}
	}
	b.StopTimer()
}
