// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestAddEndorsement(t *testing.T) {
	require := require.New(t)
	hash1 := []byte{'2', '1'}
	hash2 := []byte{'1', '2'}
	set := NewSet(hash1)
	// Successfully add an endorsement
	cv := NewConsensusVote(hash1, 1, 2, PROPOSAL)
	en := NewEndorsement(cv, testaddress.Keyinfo["producer"].PriKey, testaddress.Addrinfo["producer"].String())
	require.NoError(set.AddEndorsement(en))
	require.Equal(1, len(set.endorsements))
	// Add an endorsement with from a different endorser
	cv = NewConsensusVote(hash1, 1, 2, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Keyinfo["alfa"].PriKey, testaddress.Addrinfo["alfa"].String())
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with different hash
	cv = NewConsensusVote(hash2, 1, 2, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Keyinfo["producer"].PriKey, testaddress.Addrinfo["producer"].String())
	require.Equal(ErrInvalidHash, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with expired round number
	cv = NewConsensusVote(hash1, 1, 1, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Keyinfo["producer"].PriKey, testaddress.Addrinfo["producer"].String())
	require.Equal(ErrExpiredEndorsement, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement with advance round number
	cv = NewConsensusVote(hash1, 1, 3, PROPOSAL)
	en = NewEndorsement(cv, testaddress.Keyinfo["producer"].PriKey, testaddress.Addrinfo["producer"].String())
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(2, len(set.endorsements))
	// Add an endorsement of an existing endorser
	cv = NewConsensusVote(hash1, 1, 2, LOCK)
	en = NewEndorsement(cv, testaddress.Keyinfo["alfa"].PriKey, testaddress.Addrinfo["alfa"].String())
	require.Equal(nil, set.AddEndorsement(en))
	require.Equal(3, len(set.endorsements))
	require.Equal(1, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		LOCK: true,
	}, []string{
		testaddress.Addrinfo["producer"].String(),
		testaddress.Addrinfo["alfa"].String(),
	}))
	require.Equal(2, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		PROPOSAL: true,
	}, []string{
		testaddress.Addrinfo["producer"].String(),
		testaddress.Addrinfo["alfa"].String(),
	}))
	require.Equal(2, set.NumOfValidEndorsements(map[ConsensusVoteTopic]bool{
		LOCK:     true,
		PROPOSAL: true,
	}, []string{
		testaddress.Addrinfo["producer"].String(),
		testaddress.Addrinfo["alfa"].String(),
	}))
}
