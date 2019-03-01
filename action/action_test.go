// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestActionProto(t *testing.T) {
	require := require.New(t)
	v, err := NewVote(0, testaddress.Addrinfo["bravo"].String(),
		uint64(100000), big.NewInt(10))
	require.NoError(err)
	fmt.Println(v)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).
		SetGasLimit(uint64(100000)).
		SetAction(v).Build()

	selp, err := Sign(elp, testaddress.Keyinfo["alfa"].PriKey)
	require.NoError(err)

	require.NoError(Verify(selp))

	nselp := &SealedEnvelope{}
	require.NoError(nselp.LoadProto(selp.Proto()))

	require.Equal(selp.Hash(), nselp.Hash())
}
