// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"testing"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/require"
)

func TestAddEndorsement(t *testing.T) {
	require := require.New(t)
	hash1 := byteutil.BytesTo32B([]byte{'2', '1'})
	hash2 := byteutil.BytesTo32B([]byte{'1', '2'})
	set := NewSet(hash1)
	// Successfully add an endorsement
	cv := NewConsensusVote(hash1, 1, 2, PROPOSAL)
	en := NewEndorsement(cv, testaddress.Addrinfo["producer"])
	require.NoError(set.AddEndorsement(en))
	require.Equal(1, len(set.endorsements))
	// Add an endorsement with from a different endorser
	cv = NewConsensusVote(hash1, 1, 2, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Addrinfo["alfa"])
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with different hash
	cv = NewConsensusVote(hash2, 1, 2, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Addrinfo["producer"])
	require.Equal(ErrInvalidHash, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with expired round number
	cv = NewConsensusVote(hash1, 1, 1, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Addrinfo["producer"])
	require.Equal(ErrExpiredEndorsement, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with advance round number
	cv = NewConsensusVote(hash1, 1, 3, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Addrinfo["producer"])
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement of an existing endorser
	cv = NewConsensusVote(hash1, 1, 2, LOCK)
	en = NewEndorsement(cv, testaddress.Addrinfo["alfa"])
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(3, len(set.endorsements))
	require.Equal(1, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		LOCK: true,
	}, []string{
		testaddress.Addrinfo["producer"].RawAddress,
		testaddress.Addrinfo["alfa"].RawAddress,
	}))
	require.Equal(2, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		PROPOSAL: true,
	}, []string{
		testaddress.Addrinfo["producer"].RawAddress,
		testaddress.Addrinfo["alfa"].RawAddress,
	}))
	require.Equal(2, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		LOCK:     true,
		PROPOSAL: true,
	}, []string{
		testaddress.Addrinfo["producer"].RawAddress,
		testaddress.Addrinfo["alfa"].RawAddress,
	}))
}
