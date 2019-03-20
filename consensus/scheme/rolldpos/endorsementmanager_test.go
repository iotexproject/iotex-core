// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEndorserEndorsementCollection(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	priKey := identityset.PrivateKey(0)
	mockProposal := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	mockLock := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	mockCommit := endorsement.NewEndorsement(
		now.Add(-2*time.Second),
		priKey.PublicKey(),
		[]byte{},
	)
	eec := newEndorserEndorsementCollection()
	require.NoError(eec.AddEndorsement(PROPOSAL, mockProposal))
	require.NoError(eec.AddEndorsement(LOCK, mockLock))
	require.NoError(eec.AddEndorsement(COMMIT, mockCommit))
	t.Run("read", func(t *testing.T) {
		require.Equal(mockProposal, eec.Endorsement(PROPOSAL))
		require.Equal(mockLock, eec.Endorsement(LOCK))
		require.Equal(mockCommit, eec.Endorsement(COMMIT))
	})
	t.Run("cleanup", func(t *testing.T) {
		cleaned := eec.Cleanup(now)
		require.Nil(cleaned.Endorsement(PROPOSAL))
		require.Nil(cleaned.Endorsement(LOCK))
		require.NotNil(cleaned.Endorsement(COMMIT))
	})
	t.Run("failure-to-replace", func(t *testing.T) {
		mockProposal2 := endorsement.NewEndorsement(
			now.Add(-3*time.Second),
			priKey.PublicKey(),
			[]byte{},
		)
		require.Error(eec.AddEndorsement(PROPOSAL, mockProposal2))
	})
	t.Run("success-to-replace", func(t *testing.T) {
		mockProposal2 := endorsement.NewEndorsement(
			now.Add(-2*time.Second),
			priKey.PublicKey(),
			[]byte{},
		)
		require.NoError(eec.AddEndorsement(PROPOSAL, mockProposal2))
	})
}

func TestBlockEndorsementCollection(t *testing.T) {

}

func TestEndorsementManager(t *testing.T) {

}
