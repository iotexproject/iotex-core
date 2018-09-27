// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

func TestCandidate(t *testing.T) {
	require := require.New(t)

	cand1 := &Candidate{
		Address: "io1qyqsyqcy0av2reec8lrrth063uj8xkuugmmtu3tm2pahu9",
		Votes:   big.NewInt(1),
	}
	cand2 := &Candidate{
		Address: "io1qyqsyqcyl7ge4df3g94rzxvd2acldsfvdtq22kvf2ws722",
		Votes:   big.NewInt(2),
	}
	cand3 := &Candidate{
		Address: "io1qyqsyqcyf64rhvaj2y70q66yzkrpkhl52428vm5v88gqah",
		Votes:   big.NewInt(3),
	}

	pkHash, err := iotxaddress.GetPubkeyHash(cand1.Address)
	require.NoError(err)
	cand1Hash := byteutil.BytesTo20B(pkHash)

	pkHash, err = iotxaddress.GetPubkeyHash(cand2.Address)
	require.NoError(err)
	cand2Hash := byteutil.BytesTo20B(pkHash)

	pkHash, err = iotxaddress.GetPubkeyHash(cand3.Address)
	require.NoError(err)
	cand3Hash := byteutil.BytesTo20B(pkHash)

	candidateMap := make(map[hash.PKHash]*Candidate)
	candidateMap[cand1Hash] = cand1
	candidateMap[cand2Hash] = cand2
	candidateMap[cand3Hash] = cand3

	candidateList, err := MapToCandidates(candidateMap)
	require.NoError(err)
	require.Equal(3, len(candidateList))
	sort.Sort(candidateList)

	require.Equal("io1qyqsyqcyf64rhvaj2y70q66yzkrpkhl52428vm5v88gqah", candidateList[0].Address)
	require.Equal("io1qyqsyqcyl7ge4df3g94rzxvd2acldsfvdtq22kvf2ws722", candidateList[1].Address)
	require.Equal("io1qyqsyqcy0av2reec8lrrth063uj8xkuugmmtu3tm2pahu9", candidateList[2].Address)

	candidatesBytes, err := Serialize(candidateList)
	require.NoError(err)
	candidates, err := Deserialize(candidatesBytes)
	require.NoError(err)
	require.Equal(3, len(candidates))
	require.Equal("io1qyqsyqcyf64rhvaj2y70q66yzkrpkhl52428vm5v88gqah", candidates[0].Address)
	require.Equal("io1qyqsyqcyl7ge4df3g94rzxvd2acldsfvdtq22kvf2ws722", candidates[1].Address)
	require.Equal("io1qyqsyqcy0av2reec8lrrth063uj8xkuugmmtu3tm2pahu9", candidates[2].Address)

	candidateMap, err = CandidatesToMap(candidateList)
	require.NoError(err)
	require.Equal(3, len(candidateMap))
	require.Equal(uint64(1), candidateMap[cand1Hash].Votes.Uint64())
	require.Equal(uint64(2), candidateMap[cand2Hash].Votes.Uint64())
	require.Equal(uint64(3), candidateMap[cand3Hash].Votes.Uint64())
}
