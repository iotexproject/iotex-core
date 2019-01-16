// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

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

	vote1, err := action.NewVote(1, a.Bech32(), b.Bech32(), 0, big.NewInt(13))
	require.Nil(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetGasPrice(big.NewInt(13)).
		SetAction(vote1).
		SetDestinationAddress(b.Bech32()).Build()
	selp1, err := action.Sign(elp, a.Bech32(), priKeyA)
	require.Nil(err)

	vote2, err := action.NewVote(2, "1", "2", 0, big.NewInt(30))
	require.Nil(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(30)).
		SetAction(vote2).
		SetDestinationAddress(b.Bech32()).Build()
	selp2, err := action.Sign(elp, a.Bech32(), priKeyA)
	require.Nil(err)

	accMap[vote1.SrcAddr()] = []action.SealedEnvelope{selp1, selp2}

	tsf1, err := action.NewTransfer(uint64(1), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(15))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(15)).
		SetAction(tsf1).
		SetDestinationAddress(c.Bech32()).Build()
	selp3, err := action.Sign(elp, b.Bech32(), priKeyB)
	require.Nil(err)

	tsf2, err := action.NewTransfer(uint64(2), big.NewInt(100), "2", "3", nil, uint64(0), big.NewInt(10))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf2).
		SetDestinationAddress(c.Bech32()).Build()
	selp4, err := action.Sign(elp, b.Bech32(), priKeyB)
	require.Nil(err)

	vote3, err := action.NewVote(3, "2", "3", 0, big.NewInt(20))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(3).
		SetGasPrice(big.NewInt(20)).
		SetAction(vote3).
		SetDestinationAddress(c.Bech32()).Build()
	selp5, err := action.Sign(elp, b.Bech32(), priKeyB)
	require.Nil(err)

	accMap[tsf1.SrcAddr()] = []action.SealedEnvelope{selp3, selp4, selp5}

	tsf3, err := action.NewTransfer(uint64(1), big.NewInt(100), "3", "1", nil, uint64(0), big.NewInt(5))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetGasPrice(big.NewInt(5)).
		SetAction(tsf3).
		SetDestinationAddress(a.Bech32()).Build()
	selp6, err := action.Sign(elp, c.Bech32(), priKeyC)
	require.Nil(err)

	accMap[tsf3.SrcAddr()] = []action.SealedEnvelope{selp6}

	ai := NewActionIterator(accMap)
	act := ai.TopAction()
	require.Equal(act, selp3) // tsf1
	ai.LoadNextAction()
	act = ai.TopAction()
	require.Equal(act, selp1) // vote1
	ai.PopAction()
	act = ai.TopAction()
	require.Equal(act, selp4) //tsf2
}
