// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actioniterator

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestActionIterator(t *testing.T) {
	require := require.New(t)

	a := testaddress.Addrinfo["alfa"]
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"]
	priKeyB := testaddress.Keyinfo["bravo"].PriKey
	c := testaddress.Addrinfo["charlie"]
	priKeyC := testaddress.Keyinfo["charlie"].PriKey
	accMap := make(map[string][]action.SealedEnvelope)
	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), b.String(), nil, uint64(0), big.NewInt(13))
	require.Nil(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetGasPrice(big.NewInt(13)).
		SetAction(tsf1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.Nil(err)

	tsf2, err := action.NewTransfer(uint64(2), big.NewInt(100), "2", nil, uint64(0), big.NewInt(30))
	require.Nil(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(30)).
		SetAction(tsf2).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.Nil(err)

	accMap[a.String()] = []action.SealedEnvelope{selp1, selp2}

	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(100), c.String(), nil, uint64(0), big.NewInt(15))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(15)).
		SetAction(tsf3).Build()
	selp3, err := action.Sign(elp, priKeyB)
	require.Nil(err)

	tsf4, err := action.NewTransfer(uint64(2), big.NewInt(100), "3", nil, uint64(0), big.NewInt(10))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf4).Build()
	selp4, err := action.Sign(elp, priKeyB)
	require.Nil(err)

	tsf5, err := action.NewTransfer(uint64(3), big.NewInt(100), a.String(), nil, uint64(0), big.NewInt(2))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasPrice(big.NewInt(20)).
		SetAction(tsf5).Build()
	selp5, err := action.Sign(elp, priKeyB)
	require.Nil(err)

	accMap[b.String()] = []action.SealedEnvelope{selp3, selp4, selp5}

	tsf6, err := action.NewTransfer(uint64(1), big.NewInt(100), "1", nil, uint64(0), big.NewInt(5))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(5)).
		SetAction(tsf6).Build()
	selp6, err := action.Sign(elp, priKeyC)
	require.Nil(err)

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
