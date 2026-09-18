// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// validateBucketEndorsementWithdrawal only accepts a bucket whose endorsement
// status is exactly Endorsed; every other endorsement lifecycle state must be
// rejected with "bucket is not endorsed".
func TestValidateBucketEndorsementWithdrawal(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	v, _, err := CreateBaseView(protocol.FeatureCtx{}, sm, false)
	r.NoError(err)
	r.NoError(sm.WriteView(_protocolID, v))
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)
	esm := NewEndorsementStateManager(sm)

	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bkt := NewVoteBucket(candidate, owner, big.NewInt(10000), 1, time.Now(), false)
	bktIdx, err := csm.putBucketAndIndex(bkt)
	r.NoError(err)

	blkHeight := uint64(10)
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: blkHeight})
	ctx = protocol.WithFeatureCtx(genesis.WithGenesisContext(ctx, genesis.TestDefault()))

	// no endorsement -> not endorsed
	r.ErrorContains(validateBucketEndorsementWithdrawal(ctx, esm, bkt, blkHeight), "bucket is not endorsed")

	// endorsed indefinitely -> withdrawal validation passes
	r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
	r.Nil(validateBucketEndorsementWithdrawal(ctx, esm, bkt, blkHeight))

	// intent-to-revoke (UnEndorsing, expires later) -> not Endorsed -> rejected
	r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: blkHeight + 1}))
	r.ErrorContains(validateBucketEndorsementWithdrawal(ctx, esm, bkt, blkHeight), "bucket is not endorsed")

	// expired endorsement (EndorseExpired) -> not Endorsed -> rejected
	r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: blkHeight}))
	r.ErrorContains(validateBucketEndorsementWithdrawal(ctx, esm, bkt, blkHeight), "bucket is not endorsed")
}
