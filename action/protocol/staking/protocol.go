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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
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
	ErrAlreadyExist  = errors.New("candidate already exist")
	ErrTypeAssertion = errors.New("failed type assertion")
	TotalBucketKey   = append([]byte{_const}, []byte("totalBucket")...)
)

// Protocol defines the protocol of handling staking
type Protocol struct {
	addr       address.Address
	depositGas DepositGas
	sr         protocol.StateReader
	config     Configuration
}

// Configuration is the staking protocol configuration.
type Configuration struct {
	VoteWeightCalConsts   genesis.VoteWeightCalConsts
	RegistrationConsts    RegistrationConsts
	WithdrawWaitingPeriod time.Duration
	MinStakeAmount        *big.Int
	BootstrapCandidates   []genesis.BootstrapCandidate
}

// DepositGas deposits gas to some pool
type DepositGas func(ctx context.Context, sm protocol.StateManager, amount *big.Int) error

// NewProtocol instantiates the protocol of staking
func NewProtocol(depositGas DepositGas, sr protocol.StateReader, cfg genesis.Staking) (*Protocol, error) {
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
		sr:         sr,
	}, nil
}

// Start starts the protocol
func (p *Protocol) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
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

	center, err := getCandCenter(sm)
	if err != nil {
		return errors.Wrap(err, "failed to create CandidateStateManager")
	}

	csm, err := NewCandidateStateManager(sm, center)
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
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in CreateGenesisStates")
}

// Commit commits the last change
func (p *Protocol) Commit(ctx context.Context, sm protocol.StateManager) error {
	center, err := getCandCenter(sm)
	if err != nil {
		return errors.Wrap(err, "failed to commit candidate change in Commit")
	}

	csm, err := NewCandidateStateManager(sm, center)
	if err != nil {
		return err
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in Commit")
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	center, err := getCandCenter(sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to Handle action")
	}

	csm, err := NewCandidateStateManager(sm, center.Base())
	if err != nil {
		return nil, err
	}
	return p.handle(ctx, act, csm)
}

func (p *Protocol) handle(ctx context.Context, act action.Action, csm CandidateStateManager) (*action.Receipt, error) {
	if act == nil {
		return nil, ErrNilAction
	}
	switch act := act.(type) {
	case *action.CreateStake:
		if err := p.validateCreateStake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleCreateStake(ctx, act, csm)
	case *action.Unstake:
		if err := p.validateUnstake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleUnstake(ctx, act, csm)
	case *action.WithdrawStake:
		if err := p.validateWithdrawStake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleWithdrawStake(ctx, act, csm)
	case *action.ChangeCandidate:
		if err := p.validateChangeCandidate(ctx, act); err != nil {
			return nil, err
		}
		return p.handleChangeCandidate(ctx, act, csm)
	case *action.TransferStake:
		if err := p.validateTransferStake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleTransferStake(ctx, act, csm)
	case *action.DepositToStake:
		if err := p.validateDepositToStake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleDepositToStake(ctx, act, csm)
	case *action.Restake:
		if err := p.validateRestake(ctx, act); err != nil {
			return nil, err
		}
		return p.handleRestake(ctx, act, csm)
	case *action.CandidateRegister:
		if err := p.validateCandidateRegister(ctx, act); err != nil {
			return nil, err
		}
		return p.handleCandidateRegister(ctx, act, csm)
	case *action.CandidateUpdate:
		if err := p.validateCandidateUpdate(ctx, act); err != nil {
			return nil, err
		}
		return p.handleCandidateUpdate(ctx, act, csm)
	}
	return nil, nil
}

// ActiveCandidates returns all active candidates in candidate center
func (p *Protocol) ActiveCandidates(ctx context.Context, sr protocol.StateReader, height uint64) (state.CandidateList, error) {
	// TODO: should use createCandCenter()?
	center, err := getOrCreateCandCenter(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ActiveCandidates")
	}

	list := center.All()
	cand := make(CandidateList, 0, len(list))
	for i := range list {
		if list[i].SelfStake.Cmp(p.config.RegistrationConsts.MinSelfStake) >= 0 {
			cand = append(cand, list[i])
		}
	}
	return cand.toStateCandidateList()
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, error) {
	m := iotexapi.ReadStakingDataMethod{}
	if err := proto.Unmarshal(method, &m); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal method name")
	}
	if len(args) != 1 {
		return nil, errors.Errorf("invalid number of arguments %d", len(args))
	}
	r := iotexapi.ReadStakingDataRequest{}
	if err := proto.Unmarshal(args[0], &r); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal request")
	}

	// TODO: should use createCandCenter()?
	center, err := getOrCreateCandCenter(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get candidate center")
	}

	var resp proto.Message
	switch m.GetMethod() {
	case iotexapi.ReadStakingDataMethod_BUCKETS:
		resp, err = readStateBuckets(ctx, sr, r.GetBuckets())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER:
		resp, err = readStateBucketsByVoter(ctx, sr, r.GetBucketsByVoter())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE:
		resp, err = readStateBucketsByCandidate(ctx, sr, center, r.GetBucketsByCandidate())
	case iotexapi.ReadStakingDataMethod_CANDIDATES:
		resp, err = readStateCandidates(ctx, center, r.GetCandidates())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME:
		resp, err = readStateCandidateByName(ctx, center, r.GetCandidateByName())
	default:
		err = errors.New("corresponding method isn't found")
	}
	if err != nil {
		return nil, err
	}

	return proto.Marshal(resp)
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
