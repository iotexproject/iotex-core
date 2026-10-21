// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/slhdsa"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func qsbTestContext(blockHeight uint64, blockTime time.Time) context.Context {
	g := genesis.TestDefault()
	g.QuantumSafeBucketsBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{ChainID: 4689})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    blockHeight,
		BlockTimeStamp: blockTime,
	})
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

func qsbSign(t *testing.T, priv slhdsa.PrivateKey, msg []byte) []byte {
	t.Helper()
	sig, err := slhdsa.SignDeterministic(&priv, slhdsa.NewMessage(msg), []byte(bucketguard.Context))
	require.NoError(t, err)
	return sig
}

func qsbBind(t *testing.T, p *Protocol, csm CandidateStateManager, ctx context.Context, owner address.Address, index uint64, pub, sig []byte) {
	t.Helper()
	act := action.NewBindBucketGuard(index, bucketguard.AlgSLHDSA192s, pub, sig)
	gas, err := act.IntrinsicGas()
	require.NoError(t, err)
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       owner,
		GasPrice:     testGasPrice,
		IntrinsicGas: gas,
		Nonce:        1,
	})
	rLog, tLogs, err := p.handleBindBucketGuard(ctx, act, csm)
	require.NoError(t, err)
	require.Nil(t, tLogs)
	require.NotNil(t, rLog)
}

func TestBindAndGuardedUnstakeWithdraw(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, buckets, _ := initTestStateWithHeight(t, ctrl, []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "100000000000000000000", 30, false, false, nil, 0},
	}, []*candidateConfig{
		{identityset.Address(1), identityset.Address(1), identityset.Address(1), "test"},
	}, 1)
	bucket := buckets[0]
	owner := bucket.Owner
	sponsor := identityset.Address(2)
	recipient := identityset.Address(3)
	r.NoError(setupAccount(sm, owner, 100))
	r.NoError(setupAccount(sm, sponsor, 100))
	r.NoError(setupAccount(sm, recipient, 0))

	maturity := bucket.StakeStartTime.Add(bucket.StakedDuration)
	ctx := qsbTestContext(1, maturity)
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	r.NoError(err)

	pub, priv, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHA2_192s)
	r.NoError(err)
	pubBytes, err := pub.MarshalBinary()
	r.NoError(err)
	owner20, err := bucketguard.AddressFromBytes(owner.Bytes())
	r.NoError(err)
	accept := bucketguard.EncodeAccept(4689, bucket.Index, 1, owner20, bucketguard.AlgSLHDSA192s, pubBytes, 0)
	bindSig := qsbSign(t, priv, accept)

	qsbBind(t, p, csm, ctx, owner, bucket.Index, pubBytes, bindSig)

	guard, err := NewBucketGuardStateReader(sm).Get(bucket.Index)
	r.NoError(err)
	r.NotNil(guard)
	r.True(guard.Guarded)
	r.EqualValues(1, guard.Generation)
	r.EqualValues(0, guard.Nonce)
	r.EqualValues(1, guard.BoundHeight)

	op := &bucketguard.GuardedBucketOp{
		BucketIndex: bucket.Index,
		Generation:  1,
		Nonce:       0,
		Deadline:    1,
		Op:          bucketguard.OpUnstake,
	}
	call := bucketguard.EncodeCall(4689, owner20, op)
	op.Sig = qsbSign(t, priv, call)
	act := action.NewGuardedBucketOp(op)
	gas, err := act.IntrinsicGas()
	r.NoError(err)
	opCtx := protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       sponsor,
		GasPrice:     testGasPrice,
		IntrinsicGas: gas,
		Nonce:        2,
	})
	rLog, tLogs, err := p.handleGuardedBucketOp(opCtx, act, csm)
	r.NoError(err)
	r.NotNil(rLog)
	r.Nil(tLogs)

	got, err := newCandidateStateReader(sm).NativeBucket(bucket.Index)
	r.NoError(err)
	r.True(got.isUnstaked())
	guard, err = NewBucketGuardStateReader(sm).Get(bucket.Index)
	r.NoError(err)
	r.EqualValues(1, guard.Generation)
	r.EqualValues(1, guard.Nonce)

	withdrawAt := got.UnstakeStartTime.Add(15 * 24 * time.Hour)
	withdrawCtx := qsbTestContext(2, withdrawAt)
	op = &bucketguard.GuardedBucketOp{
		BucketIndex: bucket.Index,
		Generation:  1,
		Nonce:       1,
		Deadline:    2,
		Op:          bucketguard.OpWithdraw,
		Recipient:   recipient.Bytes(),
	}
	call = bucketguard.EncodeCall(4689, owner20, op)
	op.Sig = qsbSign(t, priv, call)
	act = action.NewGuardedBucketOp(op)
	gas, err = act.IntrinsicGas()
	r.NoError(err)
	withdrawCtx = protocol.WithActionCtx(withdrawCtx, protocol.ActionCtx{
		Caller:       sponsor,
		GasPrice:     testGasPrice,
		IntrinsicGas: gas,
		Nonce:        3,
	})
	rLog, tLogs, err = p.handleGuardedBucketOp(withdrawCtx, act, csm)
	r.NoError(err)
	r.NotNil(rLog)
	r.Len(tLogs, 1)
	r.Equal(iotextypes.TransactionLogType_WITHDRAW_BUCKET, tLogs[0].Type)
	r.Equal(recipient.String(), tLogs[0].Recipient)

	_, err = newCandidateStateReader(sm).NativeBucket(bucket.Index)
	r.ErrorIs(err, state.ErrStateNotExist)
	guard, err = NewBucketGuardStateReader(sm).Get(bucket.Index)
	r.NoError(err)
	r.Nil(guard)
	account, err := accountutil.LoadAccount(sm, recipient)
	r.NoError(err)
	r.Equal(bucket.StakedAmount.String(), account.Balance.String())
}

func TestGuardedTransferClearAndRotate(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm, p, buckets, _ := initTestStateWithHeight(t, ctrl, []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "100000000000000000000", 30, false, false, nil, 0},
	}, []*candidateConfig{
		{identityset.Address(1), identityset.Address(1), identityset.Address(1), "test"},
	}, 1)
	bucket := buckets[0]
	owner := bucket.Owner
	newOwner := identityset.Address(2)
	sponsor := identityset.Address(3)
	r.NoError(setupAccount(sm, owner, 100))
	r.NoError(setupAccount(sm, newOwner, 100))
	r.NoError(setupAccount(sm, sponsor, 100))

	ctx := qsbTestContext(1, bucket.StakeStartTime)
	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	r.NoError(err)

	pub, priv, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHA2_192s)
	r.NoError(err)
	pubBytes, err := pub.MarshalBinary()
	r.NoError(err)
	owner20, err := bucketguard.AddressFromBytes(owner.Bytes())
	r.NoError(err)
	accept := bucketguard.EncodeAccept(4689, bucket.Index, 1, owner20, bucketguard.AlgSLHDSA192s, pubBytes, 0)
	qsbBind(t, p, csm, ctx, owner, bucket.Index, pubBytes, qsbSign(t, priv, accept))

	newPub, newPriv, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHA2_192s)
	r.NoError(err)
	newPubBytes, err := newPub.MarshalBinary()
	r.NoError(err)
	newOwner20, err := bucketguard.AddressFromBytes(newOwner.Bytes())
	r.NoError(err)
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: bucket.Index,
		Generation:  1,
		Nonce:       0,
		Deadline:    1,
		Op:          bucketguard.OpTransfer,
		NewOwner:    newOwner.Bytes(),
		NewGuard: &bucketguard.NewGuard{
			AlgID:     bucketguard.AlgSLHDSA192s,
			PublicKey: newPubBytes,
		},
	}
	op.NewGuard.AcceptSig = qsbSign(t, newPriv, bucketguard.EncodeAccept(4689, bucket.Index, 2, newOwner20, bucketguard.AlgSLHDSA192s, newPubBytes, 1))
	op.Sig = qsbSign(t, priv, bucketguard.EncodeCall(4689, owner20, op))
	act := action.NewGuardedBucketOp(op)
	gas, err := act.IntrinsicGas()
	r.NoError(err)
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       sponsor,
		GasPrice:     testGasPrice,
		IntrinsicGas: gas,
		Nonce:        2,
	})
	_, _, err = p.handleGuardedBucketOp(ctx, act, csm)
	r.NoError(err)

	got, err := newCandidateStateReader(sm).NativeBucket(bucket.Index)
	r.NoError(err)
	r.True(address.Equal(newOwner, got.Owner))
	guard, err := NewBucketGuardStateReader(sm).Get(bucket.Index)
	r.NoError(err)
	r.EqualValues(2, guard.Generation)
	r.EqualValues(1, guard.Nonce)
	r.EqualValues(1, guard.BoundHeight)
	r.Equal(newPubBytes, guard.PublicKey)

	clearOp := &bucketguard.GuardedBucketOp{
		BucketIndex: bucket.Index,
		Generation:  2,
		Nonce:       1,
		Deadline:    2,
		Op:          bucketguard.OpTransfer,
		NewOwner:    identityset.Address(4).Bytes(),
		ClearGuard:  true,
	}
	clearOp.Sig = qsbSign(t, newPriv, bucketguard.EncodeCall(4689, newOwner20, clearOp))
	clearAct := action.NewGuardedBucketOp(clearOp)
	gas, err = clearAct.IntrinsicGas()
	r.NoError(err)
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       sponsor,
		GasPrice:     testGasPrice,
		IntrinsicGas: gas,
		Nonce:        3,
	})
	_, _, err = p.handleGuardedBucketOp(ctx, clearAct, csm)
	r.NoError(err)
	guard, err = NewBucketGuardStateReader(sm).Get(bucket.Index)
	r.NoError(err)
	r.False(guard.Guarded)
	r.EqualValues(2, guard.Generation)
	r.EqualValues(2, guard.Nonce)
	r.Empty(guard.PublicKey)
}
