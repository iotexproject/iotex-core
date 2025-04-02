// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

// TODO: pass in common and act *MigratStake, instead of elp
func (p *Protocol) handleStakeMigrate(ctx context.Context, elp action.Envelope, csm CandidateStateManager) ([]*action.Log, []*action.TransactionLog, uint64, uint64, error) {
	var (
		actLogs      = make([]*action.Log, 0)
		transferLogs = make([]*action.TransactionLog, 0)
		act          = elp.Action().(*action.MigrateStake)
		insGas, err  = act.IntrinsicGas()
	)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	gasConsumed := insGas
	gasToBeDeducted := insGas
	bucket, rErr := p.fetchBucket(csm, act.BucketIndex())
	if rErr != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, rErr
	}
	staker, rerr := fetchCaller(ctx, csm, big.NewInt(0))
	if rerr != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, errors.Wrap(rerr, "failed to fetch caller")
	}
	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, nil, gasConsumed, gasToBeDeducted, errCandNotExist
	}
	exec, err := p.constructExecution(ctx, candidate.GetIdentifier(), bucket.StakedAmount, bucket.StakedDuration, elp.Nonce(), elp.Gas(), elp.GasPrice())
	if err != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, errors.Wrap(err, "failed to construct execution")
	}
	// validate bucket index
	if err := p.validateStakeMigrate(ctx, bucket, csm); err != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, err
	}

	// snapshot for sm in case of failure of hybrid protocol handling
	si := csm.SM().Snapshot()
	revertSM := func() {
		if revertErr := csm.SM().Revert(si); revertErr != nil {
			log.L().Panic("failed to revert state", zap.Error(revertErr))
		}
	}
	// force-withdraw native bucket
	actLog, tLog, err := p.withdrawBucket(ctx, staker, bucket, candidate, csm)
	if err != nil {
		revertSM()
		return nil, nil, gasConsumed, gasToBeDeducted, err
	}
	actLogs = append(actLogs, actLog.Build(ctx, nil))
	transferLogs = append(transferLogs, tLog)
	// call staking contract to stake
	excReceipt, err := p.createNFTBucket(ctx, exec, csm.SM())
	if err != nil {
		revertSM()
		return nil, nil, gasConsumed, gasToBeDeducted, errors.Wrap(err, "failed to handle execution action")
	}
	gasConsumed += excReceipt.GasConsumed
	if excReceipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		revertSM()
		gasToBeDeducted = gasConsumed
		return nil, nil, gasConsumed, gasToBeDeducted, &handleError{
			err:           errors.Errorf("staking contract failure: %s", excReceipt.ExecutionRevertMsg()),
			failureStatus: iotextypes.ReceiptStatus(excReceipt.Status),
		}
	}
	// add sub-receipts logs
	actLogs = append(actLogs, excReceipt.Logs()...)
	transferLogs = append(transferLogs, excReceipt.TransactionLogs()...)
	return actLogs, transferLogs, gasConsumed, gasToBeDeducted, nil
}

func (p *Protocol) validateStakeMigrate(ctx context.Context, bucket *VoteBucket, csm CandidateStateManager) error {
	if err := validateBucketOwner(bucket, protocol.MustGetActionCtx(ctx).Caller); err != nil {
		return err
	}
	if !bucket.AutoStake {
		return &handleError{
			err:           errors.New("cannot migrate non-auto-staked bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	if bucket.isUnstaked() {
		return &handleError{
			err:           errors.New("cannot migrate unstaked bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	if err := validateBucketWithoutEndorsement(ctx, NewEndorsementStateManager(csm.SM()), bucket, protocol.MustGetBlockCtx(ctx).BlockHeight); err != nil {
		return err
	}
	return validateBucketSelfStake(protocol.MustGetFeatureCtx(ctx), csm, bucket, false)
}

func (p *Protocol) withdrawBucket(ctx context.Context, withdrawer *state.Account, bucket *VoteBucket, cand *Candidate, csm CandidateStateManager) (*receiptLog, *action.TransactionLog, error) {
	// delete bucket and bucket index
	if err := csm.delBucketAndIndex(bucket.Owner, bucket.Candidate, bucket.Index); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to delete bucket for candidate %s", bucket.Candidate.String())
	}

	// update bucket pool
	if err := csm.CreditBucketPool(bucket.StakedAmount); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to update staking bucket pool %s", err.Error())
	}
	// update candidate vote
	weightedVote := p.calculateVoteWeight(bucket, false)
	if err := cand.SubVote(weightedVote); err != nil {
		return nil, nil, &handleError{
			err:           errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String()),
			failureStatus: iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		}
	}
	// clear candidate's self stake if the
	if cand.SelfStakeBucketIdx == bucket.Index {
		cand.SelfStake = big.NewInt(0)
		cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	}
	if err := csm.Upsert(cand); err != nil {
		return nil, nil, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}
	// update withdrawer balance
	if err := withdrawer.AddBalance(bucket.StakedAmount); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to add balance %s", bucket.StakedAmount)
	}
	// put updated withdrawer's account state to trie
	actionCtx := protocol.MustGetActionCtx(ctx)
	if err := accountutil.StoreAccount(csm.SM(), actionCtx.Caller, withdrawer); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}
	// create receipt log
	actLog := newReceiptLog(p.addr.String(), HandleWithdrawStake, protocol.MustGetFeatureCtx(ctx).NewStakingReceiptFormat)
	actLog.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes())
	actLog.AddAddress(actionCtx.Caller)
	actLog.SetData(bucket.StakedAmount.Bytes())
	return actLog, &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
		Amount:    bucket.StakedAmount,
		Sender:    address.StakingBucketPoolAddr,
		Recipient: actionCtx.Caller.String(),
	}, nil
}

func (p *Protocol) ConstructExecution(ctx context.Context, act *action.MigrateStake, nonce, gas uint64, gasPrice *big.Int, sr protocol.StateReader) (action.Envelope, error) {
	csr, err := ConstructBaseView(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create baseview")
	}
	bucket, err := p.fetchBucket(csr, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}
	candidate := csr.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, errCandNotExist
	}
	return p.constructExecution(ctx, candidate.GetIdentifier(), bucket.StakedAmount, bucket.StakedDuration, nonce, gas, gasPrice)
}

func (p *Protocol) constructExecution(ctx context.Context, candidate address.Address, amount *big.Int, durationTime time.Duration, nonce uint64, gasLimit uint64, gasPrice *big.Int) (action.Envelope, error) {
	contractAddress := p.config.TimestampedMigrateContractAddress
	duration := uint64(durationTime.Seconds())
	if !protocol.MustGetFeatureCtx(ctx).TimestampedStakingContract {
		contractAddress = p.config.MigrateContractAddress
		duration = uint64(durationTime / p.helperCtx.BlockInterval(protocol.MustGetBlockCtx(ctx).BlockHeight))
	}
	data, err := StakingContractABI.Pack(
		"stake0",
		big.NewInt(int64(duration)),
		common.BytesToAddress(candidate.Bytes()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack data for contract call")
	}
	return (&action.EnvelopeBuilder{}).SetAction(action.NewExecution(contractAddress, amount, data)).
		SetNonce(nonce).SetGasLimit(gasLimit).SetGasPrice(gasPrice).Build(), nil
}

func (p *Protocol) createNFTBucket(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	exctPtl := execution.FindProtocol(protocol.MustGetRegistry(ctx))
	if exctPtl == nil {
		return nil, errors.New("execution protocol is not registered")
	}
	excReceipt, err := exctPtl.Handle(ctx, elp, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to handle execution action")
	}
	return excReceipt, nil
}
