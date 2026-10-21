// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/state"
)

func (p *Protocol) handleBindBucketGuard(ctx context.Context, act *action.BindBucketGuard, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if !featureCtx.EnableBucketGuard {
		return nil, nil, action.ErrInvalidAct
	}
	actionCtx := protocol.MustGetActionCtx(ctx)
	if _, err := fetchCaller(ctx, csm, big.NewInt(0)); err != nil {
		return nil, nil, err
	}

	bucket, rErr := p.fetchBucket(csm, act.BucketIndex)
	if rErr != nil {
		return nil, nil, rErr
	}
	if !address.Equal(bucket.Owner, actionCtx.Caller) {
		return nil, nil, &handleError{
			err:           errors.New("bucket owner does not match action caller"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	selfStake, err := isSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return nil, nil, err
	}
	if selfStake {
		return nil, nil, &handleError{
			err:           errors.New("self-stake bucket cannot be guarded"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardScope,
		}
	}

	guard, err := NewBucketGuardStateReader(csm.SR()).Get(bucket.Index)
	if err != nil {
		return nil, nil, err
	}
	var generation uint64
	if guard != nil {
		if guard.Guarded {
			return nil, nil, &handleError{
				err:           errors.New("bucket is already guarded"),
				failureStatus: iotextypes.ReceiptStatus_ErrGuardScope,
			}
		}
		generation = guard.Generation
	}
	if generation == math.MaxUint64 {
		return nil, nil, &handleError{
			err:           errors.New("guard generation overflow"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
		}
	}
	newGeneration := generation + 1
	chainID := protocol.MustGetBlockchainCtx(ctx).ChainID
	owner, err := bucketguard.AddressFromBytes(bucket.Owner.Bytes())
	if err != nil {
		return nil, nil, err
	}
	accept := bucketguard.EncodeAccept(chainID, bucket.Index, newGeneration, owner, act.AlgID, act.PublicKey, 0)
	if !bucketguard.Verify(act.PublicKey, accept, act.AcceptSig) {
		return nil, nil, &handleError{
			err:           errors.New("invalid ACCEPT signature"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardSignature,
		}
	}

	blockHeight := protocol.MustGetBlockCtx(ctx).BlockHeight
	if err := NewBucketGuardStateManager(csm.SM()).Put(bucket.Index, &BucketGuard{
		Guarded:     true,
		AlgID:       act.AlgID,
		PublicKey:   append([]byte(nil), act.PublicKey...),
		Generation:  newGeneration,
		Nonce:       0,
		BoundHeight: blockHeight,
	}); err != nil {
		return nil, nil, err
	}

	rLog, err := p.bucketGuardLog(bucket.Index, action.BucketGuardOpBind, newGeneration, 0, actionCtx.Caller, nil)
	if err != nil {
		return nil, nil, err
	}
	return rLog, nil, nil
}

func (p *Protocol) handleGuardedBucketOp(ctx context.Context, act *action.GuardedBucketOp, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if !featureCtx.EnableBucketGuard {
		return nil, nil, action.ErrInvalidAct
	}
	actionCtx := protocol.MustGetActionCtx(ctx)
	if _, err := fetchCaller(ctx, csm, big.NewInt(0)); err != nil {
		return nil, nil, err
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bucket, rErr := p.fetchBucket(csm, act.BucketIndex)
	if rErr != nil {
		return nil, nil, rErr
	}
	guard, err := NewBucketGuardStateReader(csm.SR()).Get(bucket.Index)
	if err != nil {
		return nil, nil, err
	}
	if guard == nil || !guard.Guarded {
		return nil, nil, &handleError{
			err:           errors.New("bucket is not guarded"),
			failureStatus: iotextypes.ReceiptStatus_ErrBucketNotGuarded,
		}
	}
	if guard.Generation != act.Generation || guard.Nonce != act.Nonce || blkCtx.BlockHeight > act.Deadline {
		return nil, nil, &handleError{
			err:           errors.New("guard generation, nonce, or deadline mismatch"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
		}
	}

	chainID := protocol.MustGetBlockchainCtx(ctx).ChainID
	owner, err := bucketguard.AddressFromBytes(bucket.Owner.Bytes())
	if err != nil {
		return nil, nil, err
	}
	if !bucketguard.Verify(guard.PublicKey, bucketguard.EncodeCall(chainID, owner, &act.GuardedBucketOp), act.Sig) {
		return nil, nil, &handleError{
			err:           errors.New("invalid CALL signature"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardSignature,
		}
	}

	newOwner := bucket.Owner
	if act.Op == bucketguard.OpTransfer {
		newOwner, err = address.FromBytes(act.NewOwner)
		if err != nil {
			return nil, nil, &handleError{
				err:           errors.Wrap(err, "invalid new owner"),
				failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
			}
		}
	}
	if act.Op == bucketguard.OpChangeCandidate {
		candidate := csm.GetByName(act.CandidateName)
		if candidate == nil {
			return nil, nil, errCandNotExist
		}
		expected, err := address.FromBytes(act.ExpectedCandidateID)
		if err != nil {
			return nil, nil, &handleError{
				err:           errors.Wrap(err, "invalid expected candidate ID"),
				failureStatus: iotextypes.ReceiptStatus_ErrCandidateMismatch,
			}
		}
		if !address.Equal(candidate.GetIdentifier(), expected) {
			return nil, nil, &handleError{
				err:           errors.New("candidate ID does not match expectedCandidateID"),
				failureStatus: iotextypes.ReceiptStatus_ErrCandidateMismatch,
			}
		}
	}

	if act.NewGuard != nil {
		if guard.Generation == math.MaxUint64 {
			return nil, nil, &handleError{
				err:           errors.New("guard generation overflow"),
				failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
			}
		}
		resultingOwner, err := bucketguard.AddressFromBytes(newOwner.Bytes())
		if err != nil {
			return nil, nil, err
		}
		accept := bucketguard.EncodeAccept(chainID, bucket.Index, guard.Generation+1, resultingOwner, act.NewGuard.AlgID, act.NewGuard.PublicKey, act.Deadline)
		if !bucketguard.Verify(act.NewGuard.PublicKey, accept, act.NewGuard.AcceptSig) {
			return nil, nil, &handleError{
				err:           errors.New("invalid ACCEPT signature"),
				failureStatus: iotextypes.ReceiptStatus_ErrGuardSignature,
			}
		}
	}

	if guard.Nonce == math.MaxUint64 {
		return nil, nil, &handleError{
			err:           errors.New("guard nonce overflow"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
		}
	}
	newNonce := guard.Nonce + 1

	var (
		rLog   *receiptLog
		tLogs  []*action.TransactionLog
		effect error
	)
	switch act.Op {
	case bucketguard.OpUnstake:
		innerCtx := p.guardAuthorizedCtx(ctx, bucket, actionCtx)
		_, effect = p.handleUnstake(innerCtx, action.NewUnstake(bucket.Index, nil), csm)
	case bucketguard.OpWithdraw:
		rLog, tLogs, effect = p.handleGuardedWithdraw(ctx, act, bucket, csm)
	case bucketguard.OpTransfer:
		transfer, err := action.NewTransferStake(newOwner.String(), bucket.Index, nil)
		if err != nil {
			return nil, nil, err
		}
		innerCtx := p.guardAuthorizedCtx(ctx, bucket, actionCtx)
		_, effect = p.handleTransferStake(innerCtx, transfer, csm)
	case bucketguard.OpChangeCandidate:
		innerCtx := p.guardAuthorizedCtx(ctx, bucket, actionCtx)
		_, effect = p.handleChangeCandidate(innerCtx, action.NewChangeCandidate(act.CandidateName, bucket.Index, nil), csm)
	case bucketguard.OpRestake:
		innerCtx := p.guardAuthorizedCtx(ctx, bucket, actionCtx)
		_, effect = p.handleRestake(innerCtx, action.NewRestake(bucket.Index, act.Duration, act.AutoStake, nil), csm)
	case bucketguard.OpRotate:
	default:
		return nil, nil, action.ErrInvalidAct
	}
	if effect != nil {
		return nil, nil, effect
	}

	bsm := NewBucketGuardStateManager(csm.SM())
	switch act.Op {
	case bucketguard.OpWithdraw:
		// handleGuardedWithdraw already deleted the record and emitted the log.
		return rLog, tLogs, nil
	case bucketguard.OpTransfer:
		if act.ClearGuard {
			if err := bsm.Put(bucket.Index, &BucketGuard{
				Guarded:    false,
				Generation: guard.Generation,
				Nonce:      newNonce,
			}); err != nil {
				return nil, nil, err
			}
		} else {
			if err := bsm.Put(bucket.Index, &BucketGuard{
				Guarded:     true,
				AlgID:       act.NewGuard.AlgID,
				PublicKey:   append([]byte(nil), act.NewGuard.PublicKey...),
				Generation:  guard.Generation + 1,
				Nonce:       newNonce,
				BoundHeight: blkCtx.BlockHeight,
			}); err != nil {
				return nil, nil, err
			}
		}
	case bucketguard.OpRotate:
		if err := bsm.Put(bucket.Index, &BucketGuard{
			Guarded:     true,
			AlgID:       act.NewGuard.AlgID,
			PublicKey:   append([]byte(nil), act.NewGuard.PublicKey...),
			Generation:  guard.Generation + 1,
			Nonce:       newNonce,
			BoundHeight: blkCtx.BlockHeight,
		}); err != nil {
			return nil, nil, err
		}
	default:
		guard.Nonce = newNonce
		if err := bsm.Put(bucket.Index, guard); err != nil {
			return nil, nil, err
		}
	}

	generation := guard.Generation
	if act.NewGuard != nil {
		generation = guard.Generation + 1
	}
	counterparty := []byte(nil)
	if act.Op == bucketguard.OpTransfer {
		counterparty = newOwner.Bytes()
	}
	rLog, err = p.bucketGuardLog(bucket.Index, uint64(act.Op), generation, newNonce, actionCtx.Caller, counterparty)
	if err != nil {
		return nil, nil, err
	}
	return rLog, nil, nil
}

func (p *Protocol) guardAuthorizedCtx(ctx context.Context, bucket *VoteBucket, actionCtx protocol.ActionCtx) context.Context {
	inner := protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       bucket.Owner,
		ActionHash:   actionCtx.ActionHash,
		GasPrice:     big.NewInt(0),
		IntrinsicGas: 0,
		Nonce:        actionCtx.Nonce,
		ReadOnly:     actionCtx.ReadOnly,
	})
	return withGuardAuthorization(inner, bucket.Index)
}

func (p *Protocol) handleGuardedWithdraw(ctx context.Context, act *action.GuardedBucketOp, bucket *VoteBucket, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	recipient, err := address.FromBytes(act.Recipient)
	if err != nil {
		return nil, nil, &handleError{
			err:           errors.Wrap(err, "invalid recipient"),
			failureStatus: iotextypes.ReceiptStatus_ErrGuardMismatch,
		}
	}
	cannotWithdraw := bucket.UnstakeStartTime.Unix() == 0
	if featureCtx.CannotUnstakeAgain {
		cannotWithdraw = !bucket.isUnstaked()
	}
	if cannotWithdraw {
		return nil, nil, &handleError{
			err:           errors.New("bucket has not been unstaked"),
			failureStatus: iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		}
	}
	withdrawWaitTime := p.config.WithdrawWaitingPeriod
	if !featureCtx.NewStakingReceiptFormat {
		withdrawWaitTime = _withdrawWaitingTime
	}
	if blkCtx.BlockTimeStamp.Before(bucket.UnstakeStartTime.Add(withdrawWaitTime)) {
		return nil, nil, &handleError{
			err:           errors.New("stake is not ready to withdraw"),
			failureStatus: iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity,
		}
	}
	if err := csm.delBucketAndIndex(bucket.Owner, bucket.Candidate, bucket.Index); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to delete bucket for candidate %s", bucket.Candidate.String())
	}
	if err := csm.CreditBucketPool(bucket.StakedAmount, true); err != nil {
		return nil, nil, &handleError{
			err:           errors.Wrapf(err, "failed to update staking bucket pool %s", err.Error()),
			failureStatus: iotextypes.ReceiptStatus_ErrWriteAccount,
		}
	}
	accountCreationOpts := []state.AccountCreationOption{}
	if featureCtx.CreateLegacyNonceAccount {
		accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
	}
	account, err := accountutil.LoadAccount(csm.SM(), recipient, accountCreationOpts...)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load recipient %s", recipient.String())
	}
	if err := account.AddBalance(bucket.StakedAmount); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to add balance %s", bucket.StakedAmount)
	}
	if err := accountutil.StoreAccount(csm.SM(), recipient, account); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to store account %s", recipient.String())
	}
	if err := NewBucketGuardStateManager(csm.SM()).Delete(bucket.Index); err != nil {
		return nil, nil, err
	}

	actionCtx := protocol.MustGetActionCtx(ctx)
	// The record was deleted above; use the values checked by the caller.
	generation := act.Generation
	nonce := act.Nonce + 1
	rLog, err := p.bucketGuardLog(bucket.Index, action.BucketGuardOpWithdraw, generation, nonce, actionCtx.Caller, recipient.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return rLog, []*action.TransactionLog{
		{
			Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
			Sender:    address.StakingBucketPoolAddr,
			Recipient: recipient.String(),
			Amount:    bucket.StakedAmount,
		},
	}, nil
}

func (p *Protocol) bucketGuardLog(bucketIndex, op, generation, nonce uint64, sponsor address.Address, counterparty []byte) (*receiptLog, error) {
	if counterparty == nil {
		counterparty = make([]byte, 20)
	}
	topics, data, err := action.PackBucketGuardEvent(bucketIndex, op, generation, nonce, sponsor.Bytes(), counterparty)
	if err != nil {
		return nil, err
	}
	rLog := newReceiptLog(p.addr.String())
	rLog.AddEvent(topics, data)
	return rLog, nil
}
