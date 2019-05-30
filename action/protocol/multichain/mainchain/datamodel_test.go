// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestSubChainState(t *testing.T) {
	t.Parallel()

	sc1 := SubChain{
		ChainID:            2,
		SecurityDeposit:    big.NewInt(1),
		OperationDeposit:   big.NewInt(2),
		StartHeight:        100,
		ParentHeightOffset: 10,
		OwnerPublicKey:     identityset.PrivateKey(27).PublicKey(),
		CurrentHeight:      200,
		DepositCount:       300,
	}
	data, err := sc1.Serialize()
	require.NoError(t, err)

	var sc2 SubChain
	require.NoError(t, sc2.Deserialize(data))
	require.Equal(t, sc1, sc2)
}

func TestBlockProofState(t *testing.T) {
	t.Parallel()

	bp1 := BlockProof{
		SubChainAddress: "123",
		Height:          123,
		ProducerAddress: "123",
		Roots: []MerkleRoot{
			{
				Name:  "abc",
				Value: hash.BytesToHash256([]byte("10002")),
			},
			{
				Name:  "abd",
				Value: hash.BytesToHash256([]byte("1000d")),
			},
		},
		ProducerPublicKey: identityset.PrivateKey(27).PublicKey(),
	}

	data, err := bp1.Serialize()
	require.NoError(t, err)

	var bp2 BlockProof
	require.NoError(t, bp2.Deserialize(data))
	require.Equal(t, bp1, bp2)
}

func TestSubChainsInOperation(t *testing.T) {
	t.Parallel()

	addr1 := hash.Hash160b([]byte{3})
	addr2 := hash.Hash160b([]byte{1})
	addr3 := hash.Hash160b([]byte{2})
	var sc1 SubChainsInOperation
	sc1 = sc1.Append(
		InOperation{
			ID:   3,
			Addr: addr1[:],
		},
	)
	sc1 = sc1.Append(
		InOperation{
			ID:   1,
			Addr: addr2[:],
		},
	)
	sc1 = sc1.Append(
		InOperation{
			ID:   2,
			Addr: addr3[:],
		},
	)

	bytes, err := sc1.Serialize()
	require.NoError(t, err)
	var sc2 SubChainsInOperation
	require.NoError(t, sc2.Deserialize(bytes))

	for i := 1; i <= 3; i++ {
		_, ok := sc2.Get(uint32(i))
		assert.True(t, ok)
	}
}
