// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/keypair"
)

func TestConstructAddress(t *testing.T) {
	require := require.New(t)
	pubkey := "2c9ccbeb9ee91271f7e5c2103753be9c9edff847e1a51227df6a6b0765f31a4b424e84027b44a663950f013a88b8fd8cdc53b1eda1d4b73f9d9dc12546c8c87d68ff1435a0f8a006"
	prikey := "b5affb30846a00ef5aa39b57f913d70cd8cf6badd587239863cb67feacf6b9f30c34e800"
	addr := ConstructAddress(1, pubkey, prikey)
	actualPubKey, err := keypair.StringToPubKeyBytes(pubkey)
	require.Nil(err)
	actualPriKey, err := keypair.StringToPriKeyBytes(prikey)
	require.Nil(err)
	require.Equal(actualPubKey, addr.PublicKey[:])
	require.Equal(actualPriKey, addr.PrivateKey[:])
}
