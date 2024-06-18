package staking

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

func (p *Protocol) handleStakeMigrate(ctx context.Context, act *action.MigrateStake, csm CandidateStateManager) ([]*action.Log, []*action.TransactionLog, uint64, uint64, bool, error) {
	actLogs := make([]*action.Log, 0)
	transferLogs := make([]*action.TransactionLog, 0)
	nonceUpdated := false
	insGas, err := act.IntrinsicGas()
	if err != nil {
		return nil, nil, 0, 0, nonceUpdated, err
	}
	gasConsumed := insGas
	gasToBeDeducted := insGas
	bucket, rErr := p.fetchBucket(csm, act.BucketIndex())
	if rErr != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, rErr
	}
	staker, rerr := fetchCaller(ctx, csm, big.NewInt(0))
	if rerr != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, errors.Wrap(rerr, "failed to fetch caller")
	}
	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, errCandNotExist
	}
	duration := uint64(bucket.StakedDuration / p.helperCtx.BlockInterval(protocol.MustGetBlockCtx(ctx).BlockHeight))
	exec, err := p.constructExecution(candidate.GetIdentifier(), bucket.StakedAmount, duration, act.Nonce(), act.GasLimit(), act.GasPrice())
	if err != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, errors.Wrap(err, "failed to construct execution")
	}
	// validate bucket index
	if err := p.validateStakeMigrate(ctx, bucket, csm); err != nil {
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, err
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
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, err
	}
	actLogs = append(actLogs, actLog.Build(ctx, nil))
	transferLogs = append(transferLogs, tLog)
	// call staking contract to stake
	excReceipt, err := p.createNFTBucket(ctx, exec, csm.SM())
	if err != nil {
		revertSM()
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, errors.Wrap(err, "failed to handle execution action")
	}
	nonceUpdated = true
	gasConsumed += excReceipt.GasConsumed
	if excReceipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		revertSM()
		nonceUpdated = false
		gasToBeDeducted = gasConsumed
		return nil, nil, gasConsumed, gasToBeDeducted, nonceUpdated, &handleError{
			err:           errors.Errorf("staking contract failure: %s", excReceipt.ExecutionRevertMsg()),
			failureStatus: iotextypes.ReceiptStatus(excReceipt.Status),
		}
	}
	// add sub-receipts logs
	actLogs = append(actLogs, excReceipt.Logs()...)
	transferLogs = append(transferLogs, excReceipt.TransactionLogs()...)
	return actLogs, transferLogs, gasConsumed, gasToBeDeducted, nonceUpdated, nil
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
	if err := validateBucketWithoutEndorsement(NewEndorsementStateManager(csm.SM()), bucket, protocol.MustGetBlockCtx(ctx).BlockHeight); err != nil {
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

func (p *Protocol) ConstructExecution(ctx context.Context, act *action.MigrateStake, sr protocol.StateReader) (*action.Execution, error) {
	sm := protocol.NewStateManagerWrapper(sr)
	csm, err := p.constructCandidateStateManager(ctx, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct state managers")
	}
	bucket, err := p.fetchBucket(csm, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}
	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, errCandNotExist
	}
	duration := uint64(bucket.StakedDuration / p.helperCtx.BlockInterval(protocol.MustGetBlockCtx(ctx).BlockHeight))

	return p.constructExecution(candidate.GetIdentifier(), bucket.StakedAmount, duration, act.Nonce(), act.GasLimit(), act.GasPrice())
}

func (p *Protocol) constructExecution(candidate address.Address, amount *big.Int, duration uint64, nonce uint64, gasLimit uint64, gasPrice *big.Int) (*action.Execution, error) {
	contractAddress := p.config.MigrateContractAddress
	data, err := StakingContractABI.Pack(
		"stake0",
		big.NewInt(int64(duration)),
		common.BytesToAddress(candidate.Bytes()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack data for contract call")
	}
	return action.NewExecution(
		contractAddress,
		nonce,
		amount,
		gasLimit,
		gasPrice,
		data,
	)
}

func (p *Protocol) createNFTBucket(ctx context.Context, exeAct *action.Execution, sm protocol.StateManager) (*action.Receipt, error) {
	exctPtl := execution.FindProtocol(protocol.MustGetRegistry(ctx))
	if exctPtl == nil {
		return nil, errors.New("execution protocol is not registered")
	}
	excReceipt, err := exctPtl.Handle(ctx, exeAct, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to handle execution action")
	}
	return excReceipt, nil
}
