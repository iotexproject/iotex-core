// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// protocolID is the protocol ID
	protocolID = "staking"

	// StakingNameSpace is the bucket name for staking state
	StakingNameSpace = "Staking"

	// CandidateNameSpace is the bucket name for candidate state
	CandidateNameSpace = "Candidate"
)

const (
	// keys in the namespace StakingNameSpace are prefixed with 1-byte tag, which serves 2 purposes:
	// 1. to be able to store multiple objects under the same key (like bucket index for voter and candidate)
	// 2. can call underlying KVStore's Filter() to retrieve a certain type of objects
	_const = byte(iota)
	_bucket
	_voterIndex
	_candIndex
)

// Errors
var (
	ErrWithdrawnBucket = errors.New("the bucket is already withdrawn")
	TotalBucketKey     = append([]byte{_const}, []byte("totalBucket")...)
)

type (
	// ReceiptError indicates a non-critical error with corresponding receipt status
	ReceiptError interface {
		Error() string
		ReceiptStatus() uint64
	}

	// Protocol defines the protocol of handling staking
	Protocol struct {
		addr       address.Address
		depositGas DepositGas
		config     Configuration
		hu         config.HeightUpgrade
	}

	// Configuration is the staking protocol configuration.
	Configuration struct {
		VoteWeightCalConsts   genesis.VoteWeightCalConsts
		RegistrationConsts    RegistrationConsts
		WithdrawWaitingPeriod time.Duration
		MinStakeAmount        *big.Int
		BootstrapCandidates   []genesis.BootstrapCandidate
	}

	// DepositGas deposits gas to some pool
	DepositGas func(ctx context.Context, sm protocol.StateManager, amount *big.Int) error
)

// NewProtocol instantiates the protocol of staking
func NewProtocol(depositGas DepositGas, cfg genesis.Staking) (*Protocol, error) {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	minStakeAmount, ok := new(big.Int).SetString(cfg.MinStakeAmount, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	regFee, ok := new(big.Int).SetString(cfg.RegistrationConsts.Fee, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	minSelfStake, ok := new(big.Int).SetString(cfg.RegistrationConsts.MinSelfStake, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	return &Protocol{
		addr: addr,
		config: Configuration{
			VoteWeightCalConsts: cfg.VoteWeightCalConsts,
			RegistrationConsts: RegistrationConsts{
				Fee:          regFee,
				MinSelfStake: minSelfStake,
			},
			WithdrawWaitingPeriod: cfg.WithdrawWaitingPeriod,
			MinStakeAmount:        minStakeAmount,
			BootstrapCandidates:   cfg.BootstrapCandidates,
		},
		depositGas: depositGas,
	}, nil
}

// Start starts the protocol
func (p *Protocol) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	p.hu = config.NewHeightUpgrade(&bcCtx.Genesis)

	// load view from SR
	c, err := createCandCenter(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start staking protocol")
	}
	return c, nil
}

// CreateGenesisStates is used to setup BootstrapCandidates from genesis config.
func (p *Protocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	if len(p.config.BootstrapCandidates) == 0 {
		return nil
	}

	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return err
	}

	for _, bc := range p.config.BootstrapCandidates {
		owner, err := address.FromString(bc.OwnerAddress)
		if err != nil {
			return err
		}

		operator, err := address.FromString(bc.OperatorAddress)
		if err != nil {
			return err
		}

		reward, err := address.FromString(bc.RewardAddress)
		if err != nil {
			return err
		}

		selfStake, ok := new(big.Int).SetString(bc.SelfStakingTokens, 10)
		if !ok {
			return ErrInvalidAmount
		}
		bucket := NewVoteBucket(owner, owner, selfStake, 7, time.Now(), true)
		bucketIdx, err := putBucketAndIndex(sm, bucket)
		if err != nil {
			return err
		}
		c := &Candidate{
			Owner:              owner,
			Operator:           operator,
			Reward:             reward,
			Name:               bc.Name,
			Votes:              p.calculateVoteWeight(bucket, true),
			SelfStakeBucketIdx: bucketIdx,
			SelfStake:          selfStake,
		}

		// put in statedb and cand center
		if err := csm.Upsert(c); err != nil {
			return err
		}
		if err := csm.DebitBucketPool(selfStake, true, false); err != nil {
			return err
		}
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in CreateGenesisStates")
}

// Commit commits the last change
func (p *Protocol) Commit(ctx context.Context, sm protocol.StateManager) error {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return err
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in Commit")
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}

	return p.handle(ctx, act, csm)
}

func (p *Protocol) handle(ctx context.Context, act action.Action, csm CandidateStateManager) (*action.Receipt, error) {
	var (
		rLog        *receiptLog
		createLog   *action.Log
		depositLog  *action.Log
		withdrawLog *action.Log
		registerLog *action.Log
		err         error
		logs        []*action.Log
	)

	switch act := act.(type) {
	case *action.CreateStake:
		rLog, createLog, err = p.handleCreateStake(ctx, act, csm)
	case *action.Unstake:
		rLog, err = p.handleUnstake(ctx, act, csm)
	case *action.WithdrawStake:
		rLog, withdrawLog, err = p.handleWithdrawStake(ctx, act, csm)
	case *action.ChangeCandidate:
		rLog, err = p.handleChangeCandidate(ctx, act, csm)
	case *action.TransferStake:
		rLog, err = p.handleTransferStake(ctx, act, csm)
	case *action.DepositToStake:
		rLog, depositLog, err = p.handleDepositToStake(ctx, act, csm)
	case *action.Restake:
		rLog, err = p.handleRestake(ctx, act, csm)
	case *action.CandidateRegister:
		rLog, createLog, registerLog, err = p.handleCandidateRegister(ctx, act, csm)
	case *action.CandidateUpdate:
		rLog, err = p.handleCandidateUpdate(ctx, act, csm)
	default:
		return nil, nil
	}

	if l := rLog.Build(ctx, err); l != nil {
		logs = append(logs, l)
	}
	if err == nil {
		if createLog != nil {
			logs = append(logs, createLog)
		}
		if depositLog != nil {
			logs = append(logs, depositLog)
		}
		if withdrawLog != nil {
			logs = append(logs, withdrawLog)
		}
		if registerLog != nil {
			logs = append(logs, registerLog)
		}
		return p.settleAction(ctx, csm, uint64(iotextypes.ReceiptStatus_Success), logs)
	}

	if receiptErr, ok := err.(ReceiptError); ok {
		log.L().Debug("Non-critical error when processing staking action", zap.Error(err))
		return p.settleAction(ctx, csm, receiptErr.ReceiptStatus(), logs)
	}
	return nil, err
}

// Validate validates a staking message
func (p *Protocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	if act == nil {
		return ErrNilAction
	}
	switch act := act.(type) {
	case *action.CreateStake:
		return p.validateCreateStake(ctx, act)
	case *action.Unstake:
		return p.validateUnstake(ctx, act)
	case *action.WithdrawStake:
		return p.validateWithdrawStake(ctx, act)
	case *action.ChangeCandidate:
		return p.validateChangeCandidate(ctx, act)
	case *action.TransferStake:
		return p.validateTransferStake(ctx, act)
	case *action.DepositToStake:
		return p.validateDepositToStake(ctx, act)
	case *action.Restake:
		return p.validateRestake(ctx, act)
	case *action.CandidateRegister:
		return p.validateCandidateRegister(ctx, act)
	case *action.CandidateUpdate:
		return p.validateCandidateUpdate(ctx, act)
	}
	return nil
}

// ActiveCandidates returns all active candidates in candidate center
func (p *Protocol) ActiveCandidates(ctx context.Context, sr protocol.StateReader, height uint64) (state.CandidateList, error) {
	c, err := getOrCreateCandCenter(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ActiveCandidates")
	}

	list := c.CandCenter().All()
	cand := make(CandidateList, 0, len(list))
	for i := range list {
		if list[i].SelfStake.Cmp(p.config.RegistrationConsts.MinSelfStake) >= 0 {
			cand = append(cand, list[i])
		}
	}
	return cand.toStateCandidateList()
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, uint64, error) {
	m := iotexapi.ReadStakingDataMethod{}
	if err := proto.Unmarshal(method, &m); err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to unmarshal method name")
	}
	if len(args) != 1 {
		return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
	}
	r := iotexapi.ReadStakingDataRequest{}
	if err := proto.Unmarshal(args[0], &r); err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to unmarshal request")
	}

	c, err := getOrCreateCandCenter(sr)
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "failed to get candidate center")
	}

	var resp proto.Message
	switch m.GetMethod() {
	case iotexapi.ReadStakingDataMethod_BUCKETS:
		resp, err = readStateBuckets(ctx, sr, r.GetBuckets())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER:
		resp, err = readStateBucketsByVoter(ctx, sr, r.GetBucketsByVoter())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE:
		resp, err = readStateBucketsByCandidate(ctx, sr, c.CandCenter(), r.GetBucketsByCandidate())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES:
		resp, err = readStateBucketByIndices(ctx, sr, r.GetBucketsByIndexes())
	case iotexapi.ReadStakingDataMethod_CANDIDATES:
		resp, err = readStateCandidates(ctx, c.CandCenter(), r.GetCandidates())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME:
		resp, err = readStateCandidateByName(ctx, c.CandCenter(), r.GetCandidateByName())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS:
		resp, err = readStateCandidateByAddress(ctx, c.CandCenter(), r.GetCandidateByAddress())
	case iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT:
		resp, err = readStateTotalStakingAmount(ctx, c.BucketPool(), r.GetTotalStakingAmount())
	default:
		err = errors.New("corresponding method isn't found")
	}
	if err != nil {
		return nil, uint64(0), err
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		return nil, uint64(0), err
	}
	stateHeight, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
	}

	return data, stateHeight, nil
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return protocolID
}

func (p *Protocol) calculateVoteWeight(v *VoteBucket, selfStake bool) *big.Int {
	return calculateVoteWeight(p.config.VoteWeightCalConsts, v, selfStake)
}

// settleAccount deposits gas fee and updates caller's nonce
func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	status uint64,
	logs []*action.Log,
) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	gasFee := big.NewInt(0).Mul(actionCtx.GasPrice, big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
	if err := p.depositGas(ctx, sm, gasFee); err != nil {
		return nil, errors.Wrap(err, "failed to deposit gas")
	}
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return nil, err
	}
	// TODO: this check shouldn't be necessary
	if actionCtx.Nonce > acc.Nonce {
		acc.Nonce = actionCtx.Nonce
	}
	if err := accountutil.StoreAccount(sm, actionCtx.Caller, acc); err != nil {
		return nil, errors.Wrap(err, "failed to update nonce")
	}
	r := action.Receipt{
		Status:          status,
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}
	if len(logs) != 0 {
		r.Logs = logs
	}
	return &r, nil
}
