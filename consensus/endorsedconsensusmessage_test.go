// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	"bytes"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEndorsedConsensusMessage(t *testing.T) {
	require := require.New(t)
	hash := []byte("abcdefg")
	sig := []byte("signature")
	priKey := identityset.PrivateKey(0)
	vote := NewConsensusVote(hash, PROPOSAL)
	now := time.Now()
	en := endorsement.NewEndorsement(
		now,
		priKey.PublicKey(),
		sig,
	)
	endorsedMessage := NewEndorsedConsensusMessage(10, vote, en)
	pb, err := endorsedMessage.Proto()
	require.NoError(err)
	cem := &EndorsedConsensusMessage{}
	require.NoError(cem.LoadProto(pb, block.NewDeserializer(0)))
	require.Equal(uint64(10), cem.Height())
	cvote, ok := cem.Document().(*ConsensusVote)
	require.True(ok)
	require.Equal(PROPOSAL, cvote.Topic())
	require.Equal(0, bytes.Compare(hash, cvote.BlockHash()))
	cen := cem.Endorsement()
	require.Equal(0, bytes.Compare(sig, cen.Signature()))
	require.True(now.Equal(cen.Timestamp()))
	require.Equal(priKey.PublicKey().HexString(), en.Endorser().HexString())
}
