// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubChainState(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	bp1 := blockProof{
		ProducerPublicKey: testaddress.Addrinfo["producer"].PublicKey,
	}

	data, err := bp1.Serialize()
	require.NoError(t, err)

	var bp2 blockProof
	require.NoError(t, bp2.Deserialize(data))
	require.Equal(t, bp1, bp2)
}

func TestUsedChainIDs(t *testing.T) {
	t.Parallel()

	input := make([]uint32, 15)
	for i := range input {
		input[i] = uint32(i + 1)
	}
	for i := range input {
		j := rand.Intn(i + 1)
		input[i], input[j] = input[j], input[i]
	}

	var usedChainIDs1 UsedChainIDs
	for _, e := range input {
		usedChainIDs1 = usedChainIDs1.Append(e)
	}
	for _, e := range input {
		assert.True(t, usedChainIDs1.Exist(e))
	}
	assert.False(t, usedChainIDs1.Exist(0))
	assert.False(t, usedChainIDs1.Exist(16))

	data, err := usedChainIDs1.Serialize()
	require.NoError(t, err)
	var usedChainIDs2 UsedChainIDs
	require.NoError(t, usedChainIDs2.Deserialize(data))
	assert.Equal(t, usedChainIDs1, usedChainIDs2)
}
