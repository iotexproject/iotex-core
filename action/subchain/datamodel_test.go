// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/require"
)

func TestSubChainState(t *testing.T) {
	sc1 := SubChain{
		ChainID:            2,
		SecurityDeposit:    big.NewInt(1),
		OperationDeposit:   big.NewInt(2),
		StartHeight:        100,
		ParentHeightOffset: 10,
		OwnerPublicKey:     testaddress.Addrinfo["producer"].PublicKey,
		CurrentHeight:      200,
	}
	data, err := sc1.Serialize()
	require.NoError(t, err)

	var sc2 SubChain
	require.NoError(t, sc2.Deserialize(data))
	require.Equal(t, sc1, sc2)
}

func TestBlockProofState(t *testing.T) {
	bp1 := blockProof{
		Hash:               byteutil.BytesTo32B(hash.Hash256b([]byte{1})),
		ActionRoot:         byteutil.BytesTo32B(hash.Hash256b([]byte{2})),
		StateRoot:          byteutil.BytesTo32B(hash.Hash256b([]byte{3})),
		ProducerPublicKey:  testaddress.Addrinfo["producer"].PublicKey,
		ConfirmationHeight: 100,
	}

	data, err := bp1.Serialize()
	require.NoError(t, err)

	var bp2 blockProof
	require.NoError(t, bp2.Deserialize(data))
	require.Equal(t, bp1, bp2)
}
