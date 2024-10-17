// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actioniterator

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestActionIterator(t *testing.T) {
	require := require.New(t)

	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29)
	priKeyB := identityset.PrivateKey(29)
	c := identityset.Address(30)
	priKeyC := identityset.PrivateKey(30)
	accMap := make(map[string][]*action.SealedEnvelope)
	tsf1 := action.NewTransfer(big.NewInt(100), b.String(), nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(13)).
		SetAction(tsf1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tsf2 := action.NewTransfer(big.NewInt(100), "2", nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(2).SetGasPrice(big.NewInt(30)).
		SetAction(tsf2).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	accMap[a.String()] = []*action.SealedEnvelope{selp1, selp2}

	tsf3 := action.NewTransfer(big.NewInt(100), c.String(), nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(15)).
		SetAction(tsf3).Build()
	selp3, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	tsf4 := action.NewTransfer(big.NewInt(100), "3", nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(2).SetGasPrice(big.NewInt(10)).
		SetAction(tsf4).Build()
	selp4, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	tsf5 := action.NewTransfer(big.NewInt(100), a.String(), nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(3).SetGasPrice(big.NewInt(20)).
		SetAction(tsf5).Build()
	selp5, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	accMap[b.String()] = []*action.SealedEnvelope{selp3, selp4, selp5}

	tsf6 := action.NewTransfer(big.NewInt(100), "1", nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(5)).
		SetAction(tsf6).Build()
	selp6, err := action.Sign(elp, priKeyC)
	require.NoError(err)

	accMap[c.String()] = []*action.SealedEnvelope{selp6}

	ai := NewActionIterator(accMap)
	appliedActionList := make([]*action.SealedEnvelope, 0)
	for {
		bestAction, ok := ai.Next()
		if !ok {
			break
		}
		appliedActionList = append(appliedActionList, bestAction)
	}
	require.Equal(appliedActionList, []*action.SealedEnvelope{selp3, selp1, selp2, selp4, selp5, selp6})
}

func TestActionByPrice(t *testing.T) {
	require := require.New(t)

	s := &actionByPrice{}
	require.Equal(0, s.Len())

	tsf1 := action.NewTransfer(big.NewInt(100), "100", nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(5)).
		SetAction(tsf1).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(28))
	require.NoError(err)

	s.Push(selp)
	require.Equal(1, s.Len())

	se := s.Pop()
	require.Equal(0, s.Len())
	require.Equal(selp, se)
}

func BenchmarkLooping(b *testing.B) {
	accMap := make(map[string][]*action.SealedEnvelope)
	for i := 0; i < b.N; i++ {
		priKey, err := crypto.HexStringToPrivateKey(fmt.Sprintf("1%063x", i))
		require.NoError(b, err)
		addr := priKey.PublicKey().Address()
		require.NotNil(b, addr)
		tsf := action.NewTransfer(big.NewInt(100), "1", nil)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(int64(i))).
			SetGasLimit(100000).SetAction(tsf).Build()
		selp, err := action.Sign(elp, priKey)
		require.NoError(b, err)
		accMap[addr.String()] = []*action.SealedEnvelope{selp}
	}
	ai := NewActionIterator(accMap)
	b.ResetTimer()
	for {
		act, ok := ai.Next()
		if !ok {
			break
		}
		if act.Gas() < 30 {
			ai.PopAccount()
		}
	}
	b.StopTimer()
}
