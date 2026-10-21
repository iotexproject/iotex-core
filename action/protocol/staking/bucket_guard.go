// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// Quantum-safe staking buckets (IIP draft #86). The rules live in
// bucketguard; this file stores the guard record, wires the two actions, and
// gives the legacy handlers their guard check.

// bucketGuardEventTopic is Keccak-256("BucketGuard(uint64,uint64,uint64,uint64,address,address)")
// (IIP §11); the event data is not ABI-encoded.
var bucketGuardEventTopic = hash.BytesToHash256(common.FromHex("0x73a1cc2c5c34de8d095f5003eb096fa77f44357dd83dcea0225ca286942426ec"))

// bucketGuardOpBind is the event op tag of BindBucketGuard; guarded ops use
// their bucketguard.Op value.
const bucketGuardOpBind = 0

// bucketGuardRecord is the state form of bucketguard.Guard.
type bucketGuardRecord struct {
	bucketguard.Guard
}

// Serialize implements state.Serializer.
func (r *bucketGuardRecord) Serialize() ([]byte, error) { return r.Guard.Serialize() }

// Deserialize implements state.Deserializer.
func (r *bucketGuardRecord) Deserialize(b []byte) error { return r.Guard.Deserialize(b) }

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (r *bucketGuardRecord) Encode() (systemcontracts.GenericValue, error) {
	data, err := r.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (r *bucketGuardRecord) Decode(v systemcontracts.GenericValue) error {
	return r.Deserialize(v.PrimaryData)
}

func bucketGuardKey(index uint64) []byte {
	return append([]byte{_bucketGuard}, byteutil.Uint64ToBytesBigEndian(index)...)
}

// getBucketGuard returns the guard record of a bucket, or nil if there is none.
// Any other read error is a backend failure and must abort the block.
func getBucketGuard(sr protocol.StateReader, index uint64) (*bucketguard.Guard, error) {
	var r bucketGuardRecord
	_, err := sr.State(&r, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(bucketGuardKey(index)))
	switch errors.Cause(err) {
	case nil:
		return &r.Guard, nil
	case state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, errors.Wrapf(err, "failed to read guard of bucket %d", index)
	}
}

func putBucketGuard(sm protocol.StateManager, index uint64, g *bucketguard.Guard) error {
	_, err := sm.PutState(&bucketGuardRecord{Guard: *g}, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(bucketGuardKey(index)))
	return errors.Wrapf(err, "failed to write guard of bucket %d", index)
}

// deleteBucketGuardWithBucket removes a bucket's guard record, a tombstone
// included, in the same snapshot that deletes the bucket (IIP §4). Bucket
// indices are never reused, so the record would otherwise be orphaned.
func deleteBucketGuardWithBucket(ctx context.Context, sm protocol.StateManager, index uint64) error {
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil
	}
	g, err := getBucketGuard(sm, index)
	if err != nil || g == nil {
		return err
	}
	_, err = sm.DelState(protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(bucketGuardKey(index)), protocol.ObjectOption(&bucketGuardRecord{}))
	return errors.Wrapf(err, "failed to delete guard of bucket %d", index)
}

// rejectGuardedBucket is the IIP §10 check of every legacy action that would
// change a native bucket's owner, candidate, duration, auto-stake flag,
// stake status, role or existence.
func (p *Protocol) rejectGuardedBucket(ctx context.Context, csm CandidateStateManager, index uint64) error {
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil
	}
	g, err := getBucketGuard(csm.SM(), index)
	if err != nil {
		return err
	}
	return guardReceiptError(bucketguard.RejectIfGuarded(g))
}

// guardReceiptError maps a bucketguard error to its receipt status. Errors
// from outside bucketguard pass through unchanged.
func guardReceiptError(err error) error {
	if err == nil {
		return nil
	}
	for _, m := range []struct {
		target error
		status iotextypes.ReceiptStatus
	}{
		{bucketguard.ErrBucketGuarded, iotextypes.ReceiptStatus_ErrBucketGuarded},
		{bucketguard.ErrBucketNotGuarded, iotextypes.ReceiptStatus_ErrBucketNotGuarded},
		{bucketguard.ErrGuardMismatch, iotextypes.ReceiptStatus_ErrGuardMismatch},
		{bucketguard.ErrGuardSignature, iotextypes.ReceiptStatus_ErrGuardSignature},
		{bucketguard.ErrGuardScope, iotextypes.ReceiptStatus_ErrGuardScope},
		{bucketguard.ErrCandidateMismatch, iotextypes.ReceiptStatus_ErrCandidateMismatch},
		// Stateless failures are rejected before execution; one surfacing here
		// is still a deterministic verdict on the action, not a node fault.
		{bucketguard.ErrMalformed, iotextypes.ReceiptStatus_Failure},
	} {
		if errors.Is(err, m.target) {
			return &handleError{err: err, failureStatus: m.status}
		}
	}
	return err
}

func (p *Protocol) validateBindBucketGuard(ctx context.Context, _ *action.BindBucketGuard) error {
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return errors.Wrap(action.ErrInvalidAct, "bucket guard is disabled")
	}
	return nil
}

// validateGuardedBucketOp applies the staking rules of the underlying
// operation, exactly as for the legacy actions; the guard fields were checked
// by SanityCheck.
func (p *Protocol) validateGuardedBucketOp(ctx context.Context, act *action.GuardedBucketOp) error {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if !featureCtx.EnableBucketGuard {
		return errors.Wrap(action.ErrInvalidAct, "bucket guard is disabled")
	}
	op := act.Guard()
	switch op.Op {
	case bucketguard.OpRestake:
		if featureCtx.CheckStakingDurationUpperLimit && op.Duration > _stakeDurationLimit {
			return ErrDurationTooHigh
		}
	case bucketguard.OpChangeCandidate:
		if !action.IsValidCandidateName(op.CandidateName) {
			return action.ErrInvalidCanName
		}
	}
	return nil
}

// fetchNativeBucket loads a bucket for the guard actions. Unlike fetchBucket,
// only "no such bucket" is a receipt; any other read error is a backend
// failure and aborts the block (IIP §9.3).
func fetchNativeBucket(csm CandidateStateManager, index uint64) (*VoteBucket, error) {
	bucket, err := csm.NativeBucket(index)
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil, &handleError{
			err:           errors.Wrapf(err, "bucket %d does not exist", index),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		}
	default:
		return nil, errors.Wrapf(err, "failed to read bucket %d", index)
	}
	if !bucket.IsNative() {
		return nil, guardReceiptError(errors.Wrapf(bucketguard.ErrGuardScope, "bucket %d is not native", index))
	}
	return bucket, nil
}

// handleBindBucketGuard follows IIP §5. Steps 1–3 (snapshot, native bucket,
// caller is owner) are here; steps 4–7 are bucketguard.AuthorizeBind.
func (p *Protocol) handleBindBucketGuard(ctx context.Context, act *action.BindBucketGuard, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil, nil, errors.Wrap(action.ErrInvalidAct, "bucket guard is disabled")
	}
	log := newReceiptLog(p.addr.String())
	index := act.BucketIndex()

	bucket, err := fetchNativeBucket(csm, index)
	if err != nil {
		return log, nil, err
	}
	if err := validateBucketOwner(bucket, actionCtx.Caller); err != nil {
		return log, nil, err
	}
	scope := bucketguard.BindScope{SelfStake: csm.ContainsSelfStakingBucket(index)}
	switch _, err := NewEndorsementStateReader(csm.SM()).Get(index); errors.Cause(err) {
	case nil:
		scope.Endorsed = true
	case state.ErrStateNotExist:
	default:
		return log, nil, errors.Wrapf(err, "failed to read endorsement of bucket %d", index)
	}
	cur, err := getBucketGuard(csm.SM(), index)
	if err != nil {
		return log, nil, err
	}
	next, err := bucketguard.AuthorizeBind(bucketguard.ParamsV1(), protocol.MustGetBlockchainCtx(ctx).ChainID,
		bucket.Owner, scope, cur, act.Guard(), blkCtx.BlockHeight)
	if err != nil {
		return log, nil, guardReceiptError(err)
	}
	if err := putBucketGuard(csm.SM(), index, next); err != nil {
		return log, nil, err
	}
	addBucketGuardEvent(log, index, bucketGuardOpBind, next, actionCtx.Caller, nil)
	return log, nil, nil
}

// handleGuardedBucketOp follows IIP §9.2:
//
//  2. the bucket exists, is native and guarded;
//  3. generation, nonce and deadline match (bucketguard.CheckOp);
//  4. the op's staking preconditions, through the same helpers as the legacy
//     handlers, which also apply the bucket effect (step 6);
//  5. the CALL and ACCEPT signatures (bucketguard.VerifyOp);
//  9. the guard record transition (bucketguard.Next).
//
// Applying the effect together with step 4 keeps one implementation per
// operation. If step 5 then fails, the staking snapshot discards the effect
// (EnableBucketGuard requires RevertStakingStateOnFailedReceipt), so the
// outcome and the receipt status are those of the IIP order.
func (p *Protocol) handleGuardedBucketOp(ctx context.Context, act *action.GuardedBucketOp, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil, nil, errors.Wrap(action.ErrInvalidAct, "bucket guard is disabled")
	}
	log := newReceiptLog(p.addr.String())
	op := act.Guard()
	index := op.BucketIndex
	height := blkCtx.BlockHeight

	// step 2
	bucket, err := fetchNativeBucket(csm, index)
	if err != nil {
		return log, nil, err
	}
	cur, err := getBucketGuard(csm.SM(), index)
	if err != nil {
		return log, nil, err
	}
	// step 3
	if err := bucketguard.CheckOp(cur, op, height); err != nil {
		return log, nil, guardReceiptError(err)
	}
	// The CALL commits to the owner at execution time, before any effect.
	owner := bucket.Owner

	// steps 4 and 6
	var (
		tLogs        []*action.TransactionLog
		counterparty address.Address
	)
	switch op.Op {
	case bucketguard.OpUnstake:
		err = p.unstakeBucket(ctx, csm, index, bucket)
	case bucketguard.OpWithdraw:
		counterparty, err = address.FromBytes(op.Recipient)
		if err != nil {
			return log, nil, guardReceiptError(errors.Wrap(bucketguard.ErrMalformed, err.Error()))
		}
		var tLog *action.TransactionLog
		tLog, err = p.guardedWithdraw(ctx, csm, index, bucket, counterparty)
		if tLog != nil {
			tLogs = append(tLogs, tLog)
		}
	case bucketguard.OpTransfer:
		counterparty, err = address.FromBytes(op.NewOwner)
		if err != nil {
			return log, nil, guardReceiptError(errors.Wrap(bucketguard.ErrMalformed, err.Error()))
		}
		err = p.transferBucketOwner(ctx, csm, index, bucket, counterparty)
	case bucketguard.OpChangeCandidate:
		candidate := csm.GetByName(op.CandidateName)
		if candidate == nil {
			return log, nil, errCandNotExist
		}
		if err := op.CheckCandidate(candidate.GetIdentifier()); err != nil {
			return log, nil, guardReceiptError(err)
		}
		err = p.changeBucketCandidate(ctx, csm, index, bucket, candidate)
	case bucketguard.OpRestake:
		err = p.restakeBucket(ctx, csm, index, bucket, op.Duration, op.AutoStake)
	case bucketguard.OpRotate:
	default:
		err = guardReceiptError(errors.Wrapf(bucketguard.ErrMalformed, "unknown op %d", uint32(op.Op)))
	}
	if err != nil {
		return log, nil, err
	}

	// step 5
	if err := bucketguard.VerifyOp(bucketguard.ParamsV1(), protocol.MustGetBlockchainCtx(ctx).ChainID, owner, cur, op); err != nil {
		return log, nil, guardReceiptError(err)
	}

	// step 9: WITHDRAW already removed the record together with the bucket.
	next, deleted := bucketguard.Next(cur, op, height)
	if !deleted {
		if err := putBucketGuard(csm.SM(), index, next); err != nil {
			return log, nil, err
		}
	} else {
		next = &bucketguard.Guard{Generation: cur.Generation, Nonce: cur.Nonce + 1}
	}
	addBucketGuardEvent(log, index, uint32(op.Op), next, actionCtx.Caller, counterparty)
	return log, tLogs, nil
}

// guardedWithdraw withdraws bucket to recipient instead of the action sender.
func (p *Protocol) guardedWithdraw(ctx context.Context, csm CandidateStateManager, index uint64, bucket *VoteBucket, recipient address.Address,
) (*action.TransactionLog, error) {
	opts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		opts = append(opts, state.LegacyNonceAccountTypeOption())
	}
	acct, err := accountutil.LoadOrCreateAccount(csm.SM(), recipient, opts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account %s", recipient.String())
	}
	if err := p.withdrawStakedBucket(ctx, csm, index, bucket, recipient, acct); err != nil {
		return nil, err
	}
	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
		Sender:    address.StakingBucketPoolAddr,
		Recipient: recipient.String(),
		Amount:    new(big.Int).Set(bucket.StakedAmount),
	}, nil
}

// addBucketGuardEvent emits the single IIP §11 event of a successful bind or
// op: topics (BucketGuard, bucketIndex, op); data generation || nonce ||
// sponsor || counterparty, with the resulting counters.
func addBucketGuardEvent(log *receiptLog, index uint64, op uint32, g *bucketguard.Guard, sponsor, counterparty address.Address) {
	data := make([]byte, 0, 56)
	data = append(data, byteutil.Uint64ToBytesBigEndian(g.Generation)...)
	data = append(data, byteutil.Uint64ToBytesBigEndian(g.Nonce)...)
	data = append(data, sponsor.Bytes()...)
	if counterparty != nil {
		data = append(data, counterparty.Bytes()...)
	} else {
		data = append(data, make([]byte, 20)...)
	}
	log.AddEvent(action.Topics{
		bucketGuardEventTopic,
		hash.BytesToHash256(byteutil.Uint64ToBytesBigEndian(index)),
		hash.BytesToHash256(byteutil.Uint32ToBytesBigEndian(op)),
	}, data)
}

// readStateBucketGuard serves ReadStakingDataMethod_BUCKET_GUARD. A bucket
// without a record yields an all-zero BucketGuard (empty publicKey: unguarded).
func readStateBucketGuard(sr protocol.StateReader, req *iotexapi.ReadStakingDataRequest_BucketGuard) (*iotextypes.BucketGuard, error) {
	g, err := getBucketGuard(sr, req.GetIndex())
	if err != nil || g == nil {
		return &iotextypes.BucketGuard{}, err
	}
	return &iotextypes.BucketGuard{
		AlgID:       uint32(g.AlgID),
		PublicKey:   g.PublicKey,
		Generation:  g.Generation,
		Nonce:       g.Nonce,
		BoundHeight: g.BoundHeight,
	}, nil
}

// ReadBucketGuard returns the guard record of a native bucket, or nil if the
// bucket has none.
func ReadBucketGuard(sr protocol.StateReader, index uint64) (*bucketguard.Guard, error) {
	return getBucketGuard(sr, index)
}
