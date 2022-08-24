// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsensusVote(t *testing.T) {
	require := require.New(t)
	hash := []byte("abcdefg")
	vote := NewConsensusVote(hash, PROPOSAL)
	require.NotNil(vote)
	require.Equal(0, bytes.Compare(hash, vote.BlockHash()))
	require.Equal(PROPOSAL, vote.Topic())
	bp, err := vote.Proto()
	require.NoError(err)
	cvote := &ConsensusVote{}
	require.NoError(cvote.LoadProto(bp))
	require.Equal(0, bytes.Compare(hash, cvote.BlockHash()))
	require.Equal(PROPOSAL, cvote.Topic())
}
