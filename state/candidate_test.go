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

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestCandidate(t *testing.T) {
	require := require.New(t)

	cand1 := &Candidate{
		Address: testaddress.Addrinfo["alfa"].Bech32(),
		Votes:   big.NewInt(1),
	}
	cand2 := &Candidate{
		Address: testaddress.Addrinfo["bravo"].Bech32(),
		Votes:   big.NewInt(2),
	}
	cand3 := &Candidate{
		Address: testaddress.Addrinfo["charlie"].Bech32(),
		Votes:   big.NewInt(3),
	}

	cand1Hash, err := address.Bech32ToPKHash(cand1.Address)
	require.NoError(err)

	cand2Hash, err := address.Bech32ToPKHash(cand2.Address)
	require.NoError(err)

	cand3Hash, err := address.Bech32ToPKHash(cand3.Address)
	require.NoError(err)

	candidateMap := make(map[hash.PKHash]*Candidate)
	candidateMap[cand1Hash] = cand1
	candidateMap[cand2Hash] = cand2
	candidateMap[cand3Hash] = cand3

	candidateList, err := MapToCandidates(candidateMap)
	require.NoError(err)
	require.Equal(3, len(candidateList))
	sort.Sort(candidateList)

	require.Equal(testaddress.Addrinfo["charlie"].Bech32(), candidateList[0].Address)
	require.Equal(testaddress.Addrinfo["bravo"].Bech32(), candidateList[1].Address)
	require.Equal(testaddress.Addrinfo["alfa"].Bech32(), candidateList[2].Address)

	candidatesBytes, err := candidateList.Serialize()
	require.NoError(err)
	var candidates CandidateList
	err = candidates.Deserialize(candidatesBytes)
	require.NoError(err)
	require.Equal(3, len(candidates))
	require.Equal(testaddress.Addrinfo["charlie"].Bech32(), candidates[0].Address)
	require.Equal(testaddress.Addrinfo["bravo"].Bech32(), candidates[1].Address)
	require.Equal(testaddress.Addrinfo["alfa"].Bech32(), candidates[2].Address)

	candidateMap, err = CandidatesToMap(candidateList)
	require.NoError(err)
	require.Equal(3, len(candidateMap))
	require.Equal(uint64(1), candidateMap[cand1Hash].Votes.Uint64())
	require.Equal(uint64(2), candidateMap[cand2Hash].Votes.Uint64())
	require.Equal(uint64(3), candidateMap[cand3Hash].Votes.Uint64())
}
