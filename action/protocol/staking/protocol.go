// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// _protocolID is the protocol ID
	_protocolID = "staking"

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
		addr               address.Address
		depositGas         DepositGas
		config             Configuration
		candBucketsIndexer *CandidatesBucketsIndexer
		voteReviser        *VoteReviser
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
	DepositGas func(ctx context.Context, sm protocol.StateManager, amount *big.Int) (*action.TransactionLog, error)
)

// FindProtocol return a registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(_protocolID)
	if !ok {
		return nil
	}
	rp, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast rolldpos protocol")
	}
	return rp
}

// NewProtocol instantiates the protocol of staking
func NewProtocol(depositGas DepositGas, cfg genesis.Staking, candBucketsIndexer *CandidatesBucketsIndexer, reviseHeights ...uint64) (*Protocol, error) {
	h := hash.Hash160b([]byte(_protocolID))
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

	// new vote reviser, revise ate greenland
	voteReviser := NewVoteReviser(cfg.VoteWeightCalConsts, reviseHeights...)

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
		depositGas:         depositGas,
		candBucketsIndexer: candBucketsIndexer,
		voteReviser:        voteReviser,
	}, nil
}

// ProtocolAddr returns the address generated from protocol id
func ProtocolAddr() address.Address {
	return protocol.HashStringToAddress(_protocolID)
}

// Start starts the protocol
func (p *Protocol) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	height, err := sr.Height()
	if err != nil {
		return nil, err
	}

	// load view from SR
	c, _, err := CreateBaseView(sr, featureCtx.ReadStateFromDB(height))
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
	csm, err := NewCandidateStateManager(sm, false)
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
		bucketIdx, err := csm.putBucketAndIndex(bucket)
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
		if err := csm.DebitBucketPool(selfStake, true); err != nil {
			return err
		}
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in CreateGenesisStates")
}

// CreatePreStates updates state manager
func (p *Protocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	g := genesis.MustExtractGenesisContext(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if blkCtx.BlockHeight == g.GreenlandBlockHeight {
		csr, err := ConstructBaseView(sm)
		if err != nil {
			return err
		}
		if _, err = sm.PutState(csr.BaseView().bucketPool.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey)); err != nil {
			return err
		}
	}

	if p.voteReviser.NeedRevise(blkCtx.BlockHeight) {
		csm, err := NewCandidateStateManager(sm, featureWithHeightCtx.ReadStateFromDB(blkCtx.BlockHeight))
		if err != nil {
			return err
		}
		if err := p.voteReviser.Revise(csm, blkCtx.BlockHeight); err != nil {
			return err
		}
	}
	if p.candBucketsIndexer == nil {
		return nil
	}
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	currentEpochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	if currentEpochNum == 0 {
		return nil
	}
	epochStartHeight := rp.GetEpochHeight(currentEpochNum)
	if epochStartHeight != blkCtx.BlockHeight || featureCtx.SkipStakingIndexer {
		return nil
	}

	return p.handleStakingIndexer(rp.GetEpochHeight(currentEpochNum-1), sm)
}

func (p *Protocol) handleStakingIndexer(epochStartHeight uint64, sm protocol.StateManager) error {
	csr := newCandidateStateReader(sm)
	allBuckets, _, err := csr.getAllBuckets()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	buckets, err := toIoTeXTypesVoteBucketList(allBuckets)
	if err != nil {
		return err
	}
	err = p.candBucketsIndexer.PutBuckets(epochStartHeight, buckets)
	if err != nil {
		return err
	}
	all, _, err := csr.getAllCandidates()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	candidateList := toIoTeXTypesCandidateListV2(all)
	return p.candBucketsIndexer.PutCandidates(epochStartHeight, candidateList)
}

// Commit commits the last change
func (p *Protocol) Commit(ctx context.Context, sm protocol.StateManager) error {
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	height, err := sm.Height()
	if err != nil {
		return err
	}
	csm, err := NewCandidateStateManager(sm, featureWithHeightCtx.ReadStateFromDB(height))
	if err != nil {
		return err
	}

	// commit updated view
	return errors.Wrap(csm.Commit(), "failed to commit candidate change in Commit")
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	featureWithHeightCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	height, err := sm.Height()
	if err != nil {
		return nil, err
	}
	csm, err := NewCandidateStateManager(sm, featureWithHeightCtx.ReadStateFromDB(height))
	if err != nil {
		return nil, err
	}

	return p.handle(ctx, act, csm)
}

func (p *Protocol) handle(ctx context.Context, act action.Action, csm CandidateStateManager) (*action.Receipt, error) {
	var (
		rLog  *receiptLog
		tLogs []*action.TransactionLog
		err   error
		logs  []*action.Log
	)

	switch act := act.(type) {
	case *action.CreateStake:
		rLog, tLogs, err = p.handleCreateStake(ctx, act, csm)
	case *action.Unstake:
		rLog, err = p.handleUnstake(ctx, act, csm)
	case *action.WithdrawStake:
		rLog, tLogs, err = p.handleWithdrawStake(ctx, act, csm)
	case *action.ChangeCandidate:
		rLog, err = p.handleChangeCandidate(ctx, act, csm)
	case *action.TransferStake:
		rLog, err = p.handleTransferStake(ctx, act, csm)
	case *action.DepositToStake:
		rLog, tLogs, err = p.handleDepositToStake(ctx, act, csm)
	case *action.Restake:
		rLog, err = p.handleRestake(ctx, act, csm)
	case *action.CandidateRegister:
		rLog, tLogs, err = p.handleCandidateRegister(ctx, act, csm)
	case *action.CandidateUpdate:
		rLog, err = p.handleCandidateUpdate(ctx, act, csm)
	default:
		return nil, nil
	}

	if l := rLog.Build(ctx, err); l != nil {
		logs = append(logs, l)
	}
	if err == nil {
		return p.settleAction(ctx, csm.SM(), uint64(iotextypes.ReceiptStatus_Success), logs, tLogs)
	}

	if receiptErr, ok := err.(ReceiptError); ok {
		actionCtx := protocol.MustGetActionCtx(ctx)
		log.L().With(
			zap.String("actionHash", hex.EncodeToString(actionCtx.ActionHash[:]))).Debug("Failed to commit staking action", zap.Error(err))
		return p.settleAction(ctx, csm.SM(), receiptErr.ReceiptStatus(), logs, tLogs)
	}
	return nil, err
}

// Validate validates a staking message
func (p *Protocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	if act == nil {
		return action.ErrNilAction
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
	c, err := GetStakingStateReader(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ActiveCandidates")
	}

	list := c.AllCandidates()
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

	csr, err := ConstructBaseView(sr)
	if err != nil {
		return nil, 0, err
	}

	// get height arg
	inputHeight, err := sr.Height()
	if err != nil {
		return nil, 0, err
	}
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochStartHeight := rp.GetEpochHeight(rp.GetEpochNum(inputHeight))

	var (
		height uint64
		resp   proto.Message
	)
	switch m.GetMethod() {
	case iotexapi.ReadStakingDataMethod_BUCKETS:
		if epochStartHeight != 0 && p.candBucketsIndexer != nil {
			return p.candBucketsIndexer.GetBuckets(epochStartHeight, r.GetBuckets().GetPagination().GetOffset(), r.GetBuckets().GetPagination().GetLimit())
		}
		resp, height, err = csr.readStateBuckets(ctx, r.GetBuckets())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER:
		resp, height, err = csr.readStateBucketsByVoter(ctx, r.GetBucketsByVoter())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE:
		resp, height, err = csr.readStateBucketsByCandidate(ctx, r.GetBucketsByCandidate())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES:
		resp, height, err = csr.readStateBucketByIndices(ctx, r.GetBucketsByIndexes())
	case iotexapi.ReadStakingDataMethod_BUCKETS_COUNT:
		resp, height, err = csr.readStateBucketCount(ctx, r.GetBucketsCount())
	case iotexapi.ReadStakingDataMethod_CANDIDATES:
		if epochStartHeight != 0 && p.candBucketsIndexer != nil {
			return p.candBucketsIndexer.GetCandidates(epochStartHeight, r.GetCandidates().GetPagination().GetOffset(), r.GetCandidates().GetPagination().GetLimit())
		}
		resp, height, err = csr.readStateCandidates(ctx, r.GetCandidates())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME:
		resp, height, err = csr.readStateCandidateByName(ctx, r.GetCandidateByName())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS:
		resp, height, err = csr.readStateCandidateByAddress(ctx, r.GetCandidateByAddress())
	case iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT:
		resp, height, err = csr.readStateTotalStakingAmount(ctx, r.GetTotalStakingAmount())
	default:
		err = errors.New("corresponding method isn't found")
	}
	if err != nil {
		return nil, height, err
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		return nil, height, err
	}
	return data, height, nil
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return _protocolID
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
	tLogs []*action.TransactionLog,
) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	gasFee := big.NewInt(0).Mul(actionCtx.GasPrice, big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
	depositLog, err := p.depositGas(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to deposit gas")
	}
	acc, err := accountutil.LoadAccount(sm, actionCtx.Caller)
	if err != nil {
		return nil, err
	}
	if err := acc.SetNonce(actionCtx.Nonce); err != nil {
		return nil, errors.Wrap(err, "failed to set nonce")
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
	r.AddLogs(logs...).AddTransactionLogs(depositLog).AddTransactionLogs(tLogs...)
	return &r, nil
}
