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

	"github.com/iotexproject/go-pkgs/cache"
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
	addr        address.Address
	candCenters *cache.ThreadSafeLruCache
	depositGas  DepositGas
	sr          protocol.StateReader
	config      Configuration
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
		addr:        addr,
		candCenters: cache.NewThreadSafeLruCache(8),
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

// CreateGenesisStates is used to setup BootstrapCandidates from genesis config.
func (p *Protocol) CreateGenesisStates(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	if len(p.config.BootstrapCandidates) == 0 {
		return nil
	}

	csm, err := p.createCandidateStateManager(sm)
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
	return nil
}

// Commit commits the last change
func (p *Protocol) Commit(ctx context.Context, sm protocol.StateManager) error {
	center, err := p.getCandCenter(sm.ConfirmedHeight())
	if err != nil {
		return errors.Wrap(err, "failed to commit candidate change in sm, cand center does not exist")
	}

	csm, err := NewCandidateStateManager(sm, center)
	if err != nil {
		return errors.Wrap(err, "failed to commit candidate change in sm")
	}
	return csm.Commit()
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	csm, err := p.createCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}
	return p.handle(ctx, act, csm)
}

func (p *Protocol) handle(ctx context.Context, act action.Action, csm CandidateStateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.CreateStake:
		r, err := p.handleCreateStake(ctx, act, csm)
		return r, err
	case *action.Unstake:
		r, err := p.handleUnstake(ctx, act, csm)
		return r, err
	case *action.WithdrawStake:
		r, err := p.handleWithdrawStake(ctx, act, csm)
		return r, err
	case *action.ChangeCandidate:
		r, err := p.handleChangeCandidate(ctx, act, csm)
		return r, err
	case *action.TransferStake:
		r, err := p.handleTransferStake(ctx, act, csm)
		return r, err
	case *action.DepositToStake:
		r, err := p.handleDepositToStake(ctx, act, csm)
		return r, err
	case *action.Restake:
		r, err := p.handleRestake(ctx, act, csm)
		return r, err
	case *action.CandidateRegister:
		r, err := p.handleCandidateRegister(ctx, act, csm)
		return r, err
	case *action.CandidateUpdate:
		r, err := p.handleCandidateUpdate(ctx, act, csm)
		return r, err
	}
	return nil, nil
}

// Validate validates a staking message
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
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
func (p *Protocol) ActiveCandidates(ctx context.Context, height uint64) (state.CandidateList, error) {
	center, err := p.getCandCenter(height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ActiveCandidates")
	}
	switch errors.Cause(err) {
	case ErrTypeAssertion:
		{
			return nil, errors.Wrap(err, "failed to create CandidateStateManager")
		}
	case ErrNilParameters:
		{
			// TODO: pass in sr
			center, err = createCandCenter(p.sr, height)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create CandidateStateManager")
			}
			p.candCenters.Add(height, center)
		}
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
	var (
		resp proto.Message
		err  error
	)

	h, err := sr.Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SR height")
	}
	center, err := p.getCandCenter(h)
	switch errors.Cause(err) {
	case ErrTypeAssertion:
		{
			return nil, errors.Wrap(err, "failed to create CandidateStateManager")
		}
	case ErrNilParameters:
		{
			center, err = createCandCenter(sr, h)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create CandidateStateManager")
			}
			p.candCenters.Add(h, center)
		}
	}

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

func (p *Protocol) createCandidateStateManager(sm protocol.StateManager) (CandidateStateManager, error) {
	h := sm.ConfirmedHeight()
	center, err := p.getCandCenter(h)
	switch errors.Cause(err) {
	case ErrTypeAssertion:
		{
			return nil, errors.Wrap(err, "failed to create CandidateStateManager")
		}
	case ErrNilParameters:
		{
			center, err = createCandCenter(sm, h)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create CandidateStateManager")
			}
			p.candCenters.Add(h, center)
		}
	}
	return NewCandidateStateManager(sm, center.Base())
}

func (p *Protocol) getCandCenter(height uint64) (CandidateCenter, error) {
	v, hit := p.candCenters.Get(height)
	if hit {
		if center, ok := v.(CandidateCenter); ok {
			return center, nil
		}
		return nil, errors.Wrap(ErrTypeAssertion, "expecting CandidateCenter")
	}
	return nil, ErrNilParameters
}

func createCandCenter(sr protocol.StateReader, height uint64) (CandidateCenter, error) {
	all, err := loadCandidatesFromSR(sr)
	if err != nil {
		return nil, err
	}

	center := NewCandidateCenter()
	if err := center.SetDelta(all); err != nil {
		return nil, err
	}
	if err := center.Commit(); err != nil {
		return nil, err
	}
	return center, nil
}
