// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestPickAction(t *testing.T) {
	require := require.New(t)
	tsf0, _ := action.NewTransfer(
		1,
		big.NewInt(90000000),
		identityset.Address(27).String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(0))
	require.NoError(err)
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp}

	iter := actioniterator.NewActionIterator(actionMap)
	require.NotNil(iter)
	acts, err := PickAction(100000, iter)
	require.NoError(err)
	require.Equal(len(acts), len(actionMap))

	acts, err = PickAction(90000, iter)
	require.NoError(err)
	require.True(len(acts) < len(actionMap))
}
