// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestActionProto(t *testing.T) {
	require := require.New(t)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v, err := NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)
	fmt.Println(v)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).
		SetGasLimit(uint64(100000)).
		SetAction(v).Build()

	selp, err := Sign(elp, identityset.PrivateKey(28))
	require.NoError(err)

	require.NoError(Verify(selp))

	nselp := &SealedEnvelope{}
	require.NoError(nselp.LoadProto(selp.Proto()))

	require.Equal(selp.Hash(), nselp.Hash())
}
