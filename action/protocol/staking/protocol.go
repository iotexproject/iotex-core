// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	// _protocolID is the protocol ID
	_protocolID = "staking"

	// _stakingNameSpace is the bucket name for staking state
	_stakingNameSpace = state.StakingNamespace

	// _candidateNameSpace is the bucket name for candidate state
	_candidateNameSpace = state.CandidateNamespace

	// CandsMapNS is the bucket name to store candidate map
	CandsMapNS = state.CandsMapNamespace

	// MaxDurationNumber is the maximum duration number
	MaxDurationNumber = math.MaxUint64
)

const (
	// keys in the namespace StakingNameSpace are prefixed with 1-byte tag, which serves 2 purposes:
	// 1. to be able to store multiple objects under the same key (like bucket index for voter and candidate)
	// 2. can call underlying KVStore's Filter() to retrieve a certain type of objects
	_const = byte(iota)
	_bucket
	_voterIndex
	_candIndex
	_endorsement
)

// Errors
var (
	ErrWithdrawnBucket     = errors.New("the bucket is already withdrawn")
	ErrEndorsementNotExist = errors.New("the endorsement does not exist")
	TotalBucketKey         = append([]byte{_const}, []byte("totalBucket")...)
)

var (
	_nameKey     = []byte("name")
	_operatorKey = []byte("operator")
	_ownerKey    = []byte("owner")
)

type (
	// ReceiptError indicates a non-critical error with corresponding receipt status
	ReceiptError interface {
		Error() string
		ReceiptStatus() uint64
	}

	ContractStakeViewBuilder interface {
		Build(ctx context.Context, target uint64) (ContractStakeView, error)
	}

	// Protocol defines the protocol of handling staking
	Protocol struct {
		addr                     address.Address
		config                   Configuration
		candBucketsIndexer       *CandidatesBucketsIndexer
		contractStakingIndexer   ContractStakingIndexerWithBucketType
		contractStakingIndexerV2 ContractStakingIndexer
		contractStakingIndexerV3 ContractStakingIndexer
		voteReviser              *VoteReviser
		patch                    *PatchStore
		helperCtx                HelperCtx
		blockStore               BlockStore
		blocksToDurationFn       func(startHeight, endHeight, currentHeight uint64) time.Duration
	}

	// Configuration is the staking protocol configuration.
	Configuration struct {
		VoteWeightCalConsts               genesis.VoteWeightCalConsts
		RegistrationConsts                RegistrationConsts
		WithdrawWaitingPeriod             time.Duration
		MinStakeAmount                    *big.Int
		BootstrapCandidates               []genesis.BootstrapCandidate
		PersistStakingPatchBlock          uint64
		FixAliasForNonStopHeight          uint64
		SkipContractStakingViewHeight     uint64
		EndorsementWithdrawWaitingBlocks  uint64
		MigrateContractAddress            string
		TimestampedMigrateContractAddress string
		MinSelfStakeToBeActive            *big.Int
	}
	// HelperCtx is the helper context for staking protocol
	HelperCtx struct {
		BlockInterval func(uint64) time.Duration
		DepositGas    protocol.DepositGas
	}
	// Option is the option to create a protocol
	Option func(*Protocol)
)

// WithContractStakingIndexerV3 sets the contract staking indexer v3
func WithContractStakingIndexerV3(indexer ContractStakingIndexer) Option {
	return func(p *Protocol) {
		p.contractStakingIndexerV3 = indexer
		p.config.TimestampedMigrateContractAddress = indexer.ContractAddress().String()
		return
	}
}

// WithBlockStore sets the block store
func WithBlockStore(bs BlockStore) Option {
	return func(p *Protocol) {
		p.blockStore = bs
	}
}

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
func NewProtocol(
	helperCtx HelperCtx,
	cfg *BuilderConfig,
	blocksToDurationFn func(startHeight, endHeight, currentHeight uint64) time.Duration,
	candBucketsIndexer *CandidatesBucketsIndexer,
	contractStakingIndexer ContractStakingIndexerWithBucketType,
	contractStakingIndexerV2 ContractStakingIndexer,
	opts ...Option,
) (*Protocol, error) {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	minStakeAmount, ok := new(big.Int).SetString(cfg.Staking.MinStakeAmount, 10)
	if !ok {
		return nil, action.ErrInvalidAmount
	}

	minSelfStakeToBeActive, ok := new(big.Int).SetString(cfg.Staking.MinSelfStakeToBeActive, 10)
	if !ok {
		return nil, errors.Wrapf(action.ErrInvalidAmount, "invalid min self stake to be active: %s", cfg.Staking.MinSelfStakeToBeActive)
	}

	regFee, ok := new(big.Int).SetString(cfg.Staking.RegistrationConsts.Fee, 10)
	if !ok {
		return nil, action.ErrInvalidAmount
	}

	minSelfStake, ok := new(big.Int).SetString(cfg.Staking.RegistrationConsts.MinSelfStake, 10)
	if !ok {
		return nil, action.ErrInvalidAmount
	}

	// new vote reviser, revise at greenland
	voteReviser := NewVoteReviser(cfg.Revise)
	migrateContractAddress := ""
	if contractStakingIndexerV2 != nil {
		migrateContractAddress = contractStakingIndexerV2.ContractAddress().String()
	}
	p := &Protocol{
		addr: addr,
		config: Configuration{
			VoteWeightCalConsts: cfg.Staking.VoteWeightCalConsts,
			RegistrationConsts: RegistrationConsts{
				Fee:          regFee,
				MinSelfStake: minSelfStake,
			},
			WithdrawWaitingPeriod:            cfg.Staking.WithdrawWaitingPeriod,
			MinStakeAmount:                   minStakeAmount,
			MinSelfStakeToBeActive:           minSelfStakeToBeActive,
			BootstrapCandidates:              cfg.Staking.BootstrapCandidates,
			PersistStakingPatchBlock:         cfg.PersistStakingPatchBlock,
			FixAliasForNonStopHeight:         cfg.FixAliasForNonStopHeight,
			SkipContractStakingViewHeight:    cfg.SkipContractStakingViewHeight,
			EndorsementWithdrawWaitingBlocks: cfg.Staking.EndorsementWithdrawWaitingBlocks,
			MigrateContractAddress:           migrateContractAddress,
		},
		blocksToDurationFn:       blocksToDurationFn,
		candBucketsIndexer:       candBucketsIndexer,
		voteReviser:              voteReviser,
		patch:                    NewPatchStore(cfg.StakingPatchDir),
		contractStakingIndexer:   contractStakingIndexer,
		helperCtx:                helperCtx,
		contractStakingIndexerV2: contractStakingIndexerV2,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

// ProtocolAddr returns the address generated from protocol id
func ProtocolAddr() address.Address {
	return protocol.HashStringToAddress(_protocolID)
}

// Start starts the protocol
func (p *Protocol) Start(ctx context.Context, sr protocol.StateReader) (protocol.View, error) {
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

	if p.needToReadCandsMap(ctx, height) {
		name, operator, owners, err := readCandCenterStateFromStateDB(sr)
		if err != nil {
			// stateDB does not have name/operator map yet
			if name, operator, owners, err = p.patch.Read(height); err != nil {
				return nil, errors.Wrap(err, "failed to read name/operator map")
			}
		}
		if err = c.candCenter.base.loadNameOperatorMapOwnerList(name, operator, owners); err != nil {
			return nil, errors.Wrap(err, "failed to load name/operator map to cand center")
		}
	}
	if p.skipContractStakingView(height) {
		return c, nil
	}

	wg := sync.WaitGroup{}
	errChan := make(chan error, 3)
	checker := blockdao.GetChecker(ctx)
	checkIndex := func(indexer ContractStakingIndexer) error {
		if checker == nil {
			return nil
		}
		if err := indexer.Start(ctx); err != nil {
			return errors.Wrap(err, "failed to start contract staking indexer")
		}
		checkerHeight, err := checker.Height()
		if err != nil {
			return errors.Wrap(err, "failed to get checker height")
		}
		if indexer.StartHeight() > checkerHeight {
			return nil
		}
		return checker.CheckIndexer(ctx, indexer, height, func(h uint64) {
			log.L().Info("Checking contract staking indexer", zap.Uint64("height", h))
		})
	}
	buildView := func(indexer ContractStakingIndexer, callback func(ContractStakeView)) {
		if indexer == nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := checkIndex(indexer); err != nil {
				errChan <- errors.Wrap(err, "failed to check contract staking indexer")
				return
			}
			view, err := NewContractStakeViewBuilder(indexer, p.blockStore).Build(ctx, sr, height)
			if err != nil {
				errChan <- errors.Wrapf(err, "failed to create stake view for contract %s", p.contractStakingIndexer.ContractAddress())
				return
			}
			callback(view)
		}()
	}
	c.contractsStake = &contractStakeView{}
	buildView(p.contractStakingIndexer, func(view ContractStakeView) {
		c.contractsStake.v1 = view
	})
	buildView(p.contractStakingIndexerV2, func(view ContractStakeView) {
		c.contractsStake.v2 = view
	})
	buildView(p.contractStakingIndexerV3, func(view ContractStakeView) {
		c.contractsStake.v3 = view
	})
	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return nil, err
		}
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
	// TODO: set init values based on ctx
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
			return action.ErrInvalidAmount
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
	return errors.Wrap(csm.Commit(ctx), "failed to commit candidate change in CreateGenesisStates")
}

func (p *Protocol) SlashCandidate(
	ctx context.Context,
	sm protocol.StateManager,
	owner address.Address,
	amount *big.Int,
) error {
	if amount == nil || amount.Sign() <= 0 {
		return errors.New("nil or non-positive amount")
	}
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return errors.Wrap(err, "failed to create candidate state manager")
	}
	candidate := csm.GetByIdentifier(owner)
	if candidate == nil {
		return errors.Wrapf(state.ErrStateNotExist, "candidate %s does not exist", owner.String())
	}
	bucket, err := p.fetchBucket(csm, candidate.SelfStakeBucketIdx)
	if err != nil {
		return errors.Wrap(err, "failed to fetch bucket")
	}
	prevWeightedVotes := p.calculateVoteWeight(bucket, true)
	if bucket.StakedAmount.Cmp(amount) < 0 {
		return errors.Errorf("amount %s is greater than staked amount %s", amount.String(), bucket.StakedAmount.String())
	}
	bucket.StakedAmount.Sub(bucket.StakedAmount, amount)
	if err := csm.updateBucket(bucket.Index, bucket); err != nil {
		return errors.Wrapf(err, "failed to update bucket %d", bucket.Index)
	}
	if err := candidate.SubVote(prevWeightedVotes); err != nil {
		return errors.Wrapf(err, "failed to sub candidate votes")
	}
	weightedVotes := p.calculateVoteWeight(bucket, true)
	if err := candidate.AddVote(weightedVotes); err != nil {
		return errors.Wrapf(err, "failed to add candidate votes")
	}
	if err := candidate.SubSelfStake(amount); err != nil {
		return errors.Wrap(err, "failed to update self stake")
	}
	if err := csm.Upsert(candidate); err != nil {
		return errors.Wrap(err, "failed to upsert candidate")
	}
	return csm.CreditBucketPool(amount)
}

// CreatePreStates updates state manager
func (p *Protocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	var (
		g          = genesis.MustExtractGenesisContext(ctx)
		blkCtx     = protocol.MustGetBlockCtx(ctx)
		featureCtx = protocol.MustGetFeatureCtx(ctx)
	)
	if blkCtx.BlockHeight == g.GreenlandBlockHeight {
		csr, err := ConstructBaseView(sm)
		if err != nil {
			return err
		}
		view := csr.BaseView()
		if _, err = sm.PutState(view.bucketPool.total, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey)); err != nil {
			return err
		}
		view.bucketPool.EnableSMStorage()
		if err := sm.WriteView(_protocolID, view); err != nil {
			return err
		}
	}
	if blkCtx.BlockHeight == p.config.FixAliasForNonStopHeight {
		csm, err := NewCandidateStateManager(sm)
		if err != nil {
			return err
		}
		base := csm.DirtyView().candCenter.base
		owners := base.all()
		if err := base.loadNameOperatorMapOwnerList(owners, owners, nil); err != nil {
			return err
		}
	}
	if p.voteReviser.NeedRevise(blkCtx.BlockHeight) {
		csm, err := NewCandidateStateManager(sm)
		if err != nil {
			return err
		}
		if err := p.voteReviser.Revise(featureCtx, csm, blkCtx.BlockHeight); err != nil {
			return err
		}
	}
	// create pre-states for contract staking
	v, err := sm.ReadView(_protocolID)
	if err != nil {
		return err
	}
	vd := v.(*viewData)
	indexers := []ContractStakingIndexer{}
	if p.contractStakingIndexer != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexer, sm, true)
		if err != nil {
			return err
		}
		indexers = append(indexers, index)
	}
	if p.contractStakingIndexerV2 != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexerV2, sm, true)
		if err != nil {
			return err
		}
		indexers = append(indexers, index)
	}
	if p.contractStakingIndexerV3 != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexerV3, sm, true)
		if err != nil {
			return err
		}
		indexers = append(indexers, index)
	}
	if blkCtx.BlockHeight == g.ToBeEnabledBlockHeight {
		handler, err := newNFTBucketEventHandler(sm, func(bucket *contractstaking.Bucket, height uint64) *big.Int {
			vb := p.convertToVoteBucket(bucket, height)
			return p.calculateVoteWeight(vb, false)
		})
		if err != nil {
			return err
		}
		buckets := make([]map[uint64]*contractstaking.Bucket, 3)
		for i, indexer := range indexers {
			h, bs, err := indexer.ContractStakingBuckets()
			if err != nil {
				return err
			}
			if indexer.StartHeight() <= blkCtx.BlockHeight && h != blkCtx.BlockHeight-1 {
				return errors.Errorf("bucket cache height %d does not match current height %d", h, blkCtx.BlockHeight-1)
			}
			buckets[i] = bs
		}
		if err := vd.contractsStake.Migrate(handler, buckets); err != nil {
			return errors.Wrap(err, "failed to flush buckets for contract staking")
		}
	}
	if featureCtx.StoreVoteOfNFTBucketIntoView {
		brs := make([]BucketReader, len(indexers))
		for i, indexer := range indexers {
			brs[i] = indexer
		}
		if err := vd.contractsStake.CreatePreStates(ctx, brs); err != nil {
			return err
		}
		if blkCtx.BlockHeight == g.WakeBlockHeight {
			buckets := make([]map[uint64]*contractstaking.Bucket, 3)
			for i, indexer := range indexers {
				h, bs, err := indexer.ContractStakingBuckets()
				if err != nil {
					return err
				}
				if indexer.StartHeight() <= blkCtx.BlockHeight && h != blkCtx.BlockHeight-1 {
					return errors.Errorf("bucket cache height %d does not match current height %d", h, blkCtx.BlockHeight-1)
				}
				buckets[i] = bs
			}
			vd.contractsStake.Revise(buckets)
		}
	}

	if p.candBucketsIndexer == nil {
		return nil
	}
	rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx))
	if rp == nil {
		return nil
	}
	currentEpochNum := rp.GetEpochNum(blkCtx.BlockHeight)
	if currentEpochNum == 0 {
		return nil
	}
	epochStartHeight := rp.GetEpochHeight(currentEpochNum)
	if epochStartHeight != blkCtx.BlockHeight || featureCtx.SkipStakingIndexer {
		return nil
	}
	return p.handleStakingIndexer(ctx, rp.GetEpochHeight(currentEpochNum-1), sm)
}

func (p *Protocol) handleStakingIndexer(ctx context.Context, epochStartHeight uint64, sm protocol.StateManager) error {
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return err
	}
	allBuckets, _, err := csr.NativeBuckets()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	buckets, err := toIoTeXTypesVoteBucketList(sm, allBuckets)
	if err != nil {
		return err
	}
	err = p.candBucketsIndexer.PutBuckets(epochStartHeight, buckets)
	if err != nil {
		return err
	}
	cc, _, err := csr.CreateCandidateCenter()
	if err != nil {
		return err
	}
	candidateList, err := toIoTeXTypesCandidateListV2(csr, cc.All(), protocol.MustGetFeatureCtx(ctx))
	if err != nil {
		return err
	}
	return p.candBucketsIndexer.PutCandidates(epochStartHeight, candidateList)
}

// PreCommit performs pre-commit
func (p *Protocol) PreCommit(ctx context.Context, sm protocol.StateManager) error {
	height, err := sm.Height()
	if err != nil {
		return err
	}
	if !p.needToWriteCandsMap(ctx, height) {
		return nil
	}
	view, err := sm.ReadView(_protocolID)
	if err != nil {
		return err
	}
	vd := view.(*viewData)
	if !vd.IsDirty() {
		return nil
	}
	if err := vd.Commit(ctx, sm); err != nil {
		return err
	}
	return vd.candCenter.WriteToStateDB(sm)
}

// Commit commits the last change
func (p *Protocol) Commit(ctx context.Context, sm protocol.StateManager) error {
	view, err := sm.ReadView(_protocolID)
	if err != nil {
		return err
	}
	if !view.(*viewData).IsDirty() {
		return nil
	}

	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return err
	}

	// commit updated view
	return errors.Wrap(csm.Commit(ctx), "failed to commit candidate change in Commit")
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (receipt *action.Receipt, err error) {
	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return nil, err
	}
	view, err := sm.ReadView(_protocolID)
	if err != nil {
		return nil, err
	}
	snapshot := view.Snapshot()
	receipt, err = p.handle(ctx, elp, csm)
	if err != nil {
		if err := view.Revert(snapshot); err != nil {
			return nil, errors.Wrap(err, "failed to revert view")
		}
		return receipt, err
	}

	return receipt, nil
}

func (p *Protocol) handle(ctx context.Context, elp action.Envelope, csm CandidateStateManager) (*action.Receipt, error) {
	var (
		rLog              *receiptLog
		tLogs             []*action.TransactionLog
		err               error
		logs              []*action.Log
		nonceUpdateOption = updateNonce
		actionCtx         = protocol.MustGetActionCtx(ctx)
		gasConsumed       = actionCtx.IntrinsicGas
		gasToBeDeducted   = gasConsumed
	)
	switch act := elp.Action().(type) {
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
	case *action.CandidateActivate:
		rLog, tLogs, err = p.handleCandidateActivate(ctx, act, csm)
	case *action.CandidateEndorsement:
		rLog, tLogs, err = p.handleCandidateEndorsement(ctx, act, csm)
	case *action.CandidateTransferOwnership:
		rLog, tLogs, err = p.handleCandidateTransferOwnership(ctx, act, csm)
	case *action.MigrateStake:
		logs, tLogs, gasConsumed, gasToBeDeducted, err = p.handleStakeMigrate(ctx, elp, csm)
		if err == nil {
			nonceUpdateOption = noUpdateNonce
		}
	default:
		return nil, nil
	}
	if rLog != nil {
		if l := rLog.Build(ctx, err); l != nil {
			logs = append(logs, l...)
		}
	}
	if err == nil {
		return p.settleAction(ctx, csm.SM(), elp, uint64(iotextypes.ReceiptStatus_Success), logs, tLogs, gasConsumed, gasToBeDeducted, nonceUpdateOption)
	}

	if receiptErr, ok := err.(ReceiptError); ok {
		actionCtx := protocol.MustGetActionCtx(ctx)
		log.L().With(
			zap.String("actionHash", hex.EncodeToString(actionCtx.ActionHash[:]))).Debug("Failed to commit staking action", zap.Error(err))
		return p.settleAction(ctx, csm.SM(), elp, receiptErr.ReceiptStatus(), logs, tLogs, gasConsumed, gasToBeDeducted, nonceUpdateOption)
	}
	return nil, err
}

// HandleReceipt handles a receipt
func (p *Protocol) HandleReceipt(ctx context.Context, elp action.Envelope, sm protocol.StateManager, receipt *action.Receipt) error {
	featureCtx, ok := protocol.GetFeatureCtx(ctx)
	if !ok {
		return errors.New("failed to get feature context from action context")
	}
	if featureCtx.StoreVoteOfNFTBucketIntoView {
		v, err := sm.ReadView(_protocolID)
		if err != nil {
			return err
		}
		if err := v.(*viewData).contractsStake.Handle(ctx, receipt); err != nil {
			return err
		}
	}
	handler, err := newNFTBucketEventHandler(sm, func(bucket *contractstaking.Bucket, height uint64) *big.Int {
		vb := p.convertToVoteBucket(bucket, height)
		return p.calculateVoteWeight(vb, false)
	})
	if err != nil {
		return err
	}
	if p.contractStakingIndexer != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexer, sm, true)
		if err != nil {
			return err
		}
		processor := index.CreateEventProcessor(ctx, handler)
		if err := processor.ProcessReceipts(ctx, receipt); err != nil {
			return errors.Wrap(err, "failed to process receipt for contract staking indexer")
		}
	}
	if p.contractStakingIndexerV2 != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexerV2, sm, true)
		if err != nil {
			return err
		}
		processor := index.CreateEventProcessor(ctx, handler)
		if err := processor.ProcessReceipts(ctx, receipt); err != nil {
			return errors.Wrap(err, "failed to process receipt for contract staking indexer v2")
		}
	}
	if p.contractStakingIndexerV3 != nil {
		index, err := contractStakingIndexerAt(p.contractStakingIndexerV3, sm, true)
		if err != nil {
			return err
		}
		processor := index.CreateEventProcessor(ctx, handler)
		if err := processor.ProcessReceipts(ctx, receipt); err != nil {
			return errors.Wrap(err, "failed to process receipt for contract staking indexer v3")
		}
	}
	return nil
}

// Validate validates a staking message
func (p *Protocol) Validate(ctx context.Context, elp action.Envelope, sr protocol.StateReader) error {
	if elp == nil || elp.Action() == nil {
		return action.ErrNilAction
	}
	switch act := elp.Action().(type) {
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
	case *action.CandidateActivate:
		return p.validateCandidateActivate(ctx, act)
	case *action.CandidateEndorsement:
		return p.validateCandidateEndorsement(ctx, act)
	case *action.CandidateTransferOwnership:
		return p.validateCandidateTransferOwnershipAction(ctx, act)
	case *action.MigrateStake:
		return p.validateMigrateStake(ctx, act)
	}
	return nil
}

func (p *Protocol) isActiveCandidate(ctx context.Context, csr CandidiateStateCommon, cand *Candidate) (bool, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NotUseMinSelfStakeToBeActive {
		if cand.SelfStake.Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
			return false, nil
		}
	} else {
		if cand.SelfStake.Cmp(p.config.MinSelfStakeToBeActive) < 0 {
			return false, nil
		}
	}
	if featureCtx.DisableDelegateEndorsement {
		// before endorsement feature, candidates with enough amount must be active
		return true, nil
	}
	bucket, err := csr.NativeBucket(cand.SelfStakeBucketIdx)
	switch {
	case errors.Cause(err) == state.ErrStateNotExist:
		// endorse bucket has been withdrawn
		return false, nil
	case err != nil:
		return false, errors.Wrapf(err, "failed to get bucket %d", cand.SelfStakeBucketIdx)
	default:
	}
	selfStake, err := isSelfStakeBucket(featureCtx, csr, bucket)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check self-stake bucket %d", cand.SelfStakeBucketIdx)
	}
	return selfStake, nil
}

// ActiveCandidates returns all active candidates in candidate center
func (p *Protocol) ActiveCandidates(ctx context.Context, sr protocol.StateReader, height uint64) (state.CandidateList, error) {
	c, err := ConstructBaseView(sr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ActiveCandidates")
	}
	list := c.AllCandidates()
	cand := make(CandidateList, 0, len(list))
	for i := range list {
		if protocol.MustGetFeatureCtx(ctx).StoreVoteOfNFTBucketIntoView {
			var csVotes *big.Int
			csVotes, err = p.contractStakingVotesFromView(ctx, list[i].GetIdentifier(), c.BaseView())
			if err != nil {
				return nil, err
			}
			list[i].Votes.Add(list[i].Votes, csVotes)
		}
		active, err := p.isActiveCandidate(ctx, c, list[i])
		if err != nil {
			return nil, err
		}
		if active {
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

	// stakeSR is the stake state reader including native and contract staking
	indexers := []ContractStakingIndexer{}
	if p.contractStakingIndexer != nil {
		indexers = append(indexers, NewDelayTolerantIndexerWithBucketType(p.contractStakingIndexer, time.Second))
	}
	if p.contractStakingIndexerV2 != nil {
		indexers = append(indexers, NewDelayTolerantIndexer(p.contractStakingIndexerV2, time.Second))
	}
	if p.contractStakingIndexerV3 != nil {
		indexers = append(indexers, NewDelayTolerantIndexer(p.contractStakingIndexerV3, time.Second))
	}
	stakeSR, err := newCompositeStakingStateReader(p.candBucketsIndexer, sr, p.calculateVoteWeight, indexers...)
	if err != nil {
		return nil, 0, err
	}

	// get height arg
	inputHeight, err := sr.Height()
	if err != nil {
		return nil, 0, err
	}
	epochStartHeight := inputHeight
	if rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx)); rp != nil {
		epochStartHeight = rp.GetEpochHeight(rp.GetEpochNum(inputHeight))
	}
	nativeSR, err := ConstructBaseView(sr)
	if err != nil {
		return nil, 0, err
	}

	var (
		height uint64
		resp   proto.Message
	)
	switch m.GetMethod() {
	case iotexapi.ReadStakingDataMethod_BUCKETS:
		if epochStartHeight != 0 && p.candBucketsIndexer != nil {
			resp, height, err = p.candBucketsIndexer.GetBuckets(epochStartHeight, r.GetBuckets().GetPagination().GetOffset(), r.GetBuckets().GetPagination().GetLimit())
		} else {
			resp, height, err = nativeSR.readStateBuckets(ctx, r.GetBuckets())
		}
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER:
		resp, height, err = nativeSR.readStateBucketsByVoter(ctx, r.GetBucketsByVoter())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_CANDIDATE:
		resp, height, err = nativeSR.readStateBucketsByCandidate(ctx, r.GetBucketsByCandidate())
	case iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES:
		resp, height, err = nativeSR.readStateBucketByIndices(ctx, r.GetBucketsByIndexes())
	case iotexapi.ReadStakingDataMethod_BUCKETS_COUNT:
		resp, height, err = nativeSR.readStateBucketCount(ctx, r.GetBucketsCount())
	case iotexapi.ReadStakingDataMethod_CANDIDATES:
		resp, height, err = stakeSR.readStateCandidates(ctx, r.GetCandidates())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME:
		resp, height, err = stakeSR.readStateCandidateByName(ctx, r.GetCandidateByName())
	case iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS:
		resp, height, err = stakeSR.readStateCandidateByAddress(ctx, r.GetCandidateByAddress())
	case iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT:
		resp, height, err = nativeSR.readStateTotalStakingAmount(ctx, r.GetTotalStakingAmount())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS:
		resp, height, err = stakeSR.readStateBuckets(ctx, r.GetBuckets())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_VOTER:
		resp, height, err = stakeSR.readStateBucketsByVoter(ctx, r.GetBucketsByVoter())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_CANDIDATE:
		resp, height, err = stakeSR.readStateBucketsByCandidate(ctx, r.GetBucketsByCandidate())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_BY_INDEXES:
		resp, height, err = stakeSR.readStateBucketByIndices(ctx, r.GetBucketsByIndexes())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_BUCKETS_COUNT:
		resp, height, err = stakeSR.readStateBucketCount(ctx, r.GetBucketsCount())
	case iotexapi.ReadStakingDataMethod_COMPOSITE_TOTAL_STAKING_AMOUNT:
		resp, height, err = stakeSR.readStateTotalStakingAmount(ctx, r.GetTotalStakingAmount())
	case iotexapi.ReadStakingDataMethod_CONTRACT_STAKING_BUCKET_TYPES:
		resp, height, err = stakeSR.readStateContractStakingBucketTypes(ctx, r.GetContractStakingBucketTypes())
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

func (p *Protocol) convertToVoteBucket(bkt *contractstaking.Bucket, height uint64) *VoteBucket {
	vb := VoteBucket{
		Index:           0,
		StakedAmount:    bkt.StakedAmount,
		AutoStake:       bkt.UnlockedAt == MaxDurationNumber,
		Candidate:       bkt.Candidate,
		Owner:           bkt.Owner,
		ContractAddress: "",
		Timestamped:     bkt.IsTimestampBased,
	}
	if bkt.IsTimestampBased {
		vb.StakedDuration = time.Duration(bkt.StakedDuration) * time.Second
		vb.StakeStartTime = time.Unix(int64(bkt.CreatedAt), 0)
		vb.CreateTime = time.Unix(int64(bkt.CreatedAt), 0)
		if bkt.UnlockedAt != MaxDurationNumber {
			vb.StakeStartTime = time.Unix(int64(bkt.UnlockedAt), 0)
		}
		if bkt.UnstakedAt == MaxDurationNumber {
			vb.UnstakeStartTime = time.Unix(0, 0)
		} else {
			vb.UnstakeStartTime = time.Unix(int64(bkt.UnstakedAt), 0)
		}
	} else {
		vb.StakedDuration = p.blocksToDurationFn(bkt.CreatedAt, bkt.CreatedAt+bkt.StakedDuration, height)
		vb.StakedDurationBlockNumber = bkt.StakedDuration
		vb.CreateBlockHeight = bkt.CreatedAt
		vb.StakeStartBlockHeight = bkt.CreatedAt
		vb.UnstakeStartBlockHeight = bkt.UnstakedAt
		if bkt.UnlockedAt != MaxDurationNumber {
			vb.StakeStartBlockHeight = bkt.UnlockedAt
		}
	}
	if bkt.Muted {
		vb.Candidate, _ = address.FromString(address.ZeroAddress)
	}
	return &vb
}

func (p *Protocol) calculateVoteWeight(v *VoteBucket, selfStake bool) *big.Int {
	return CalculateVoteWeight(p.config.VoteWeightCalConsts, v, selfStake)
}

type nonceUpdateType bool

const (
	updateNonce   nonceUpdateType = true
	noUpdateNonce nonceUpdateType = false
)

// settleAction deposits gas fee and updates caller's nonce
func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	act action.TxDynamicGas,
	status uint64,
	logs []*action.Log,
	tLogs []*action.TransactionLog,
	gasConsumed uint64,
	gasToBeDeducted uint64,
	updateNonce nonceUpdateType,
) (*action.Receipt, error) {
	var (
		actionCtx = protocol.MustGetActionCtx(ctx)
		blkCtx    = protocol.MustGetBlockCtx(ctx)
	)
	priorityFee, baseFee, err := protocol.SplitGas(ctx, act, gasToBeDeducted)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to split gas")
	}
	depositLog, err := p.helperCtx.DepositGas(ctx, sm, baseFee, protocol.PriorityFeeOption(priorityFee))
	if err != nil {
		return nil, errors.Wrap(err, "failed to deposit gas")
	}
	if updateNonce {
		accountCreationOpts := []state.AccountCreationOption{}
		if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
			accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
		}
		acc, err := accountutil.LoadAccount(sm, actionCtx.Caller, accountCreationOpts...)
		if err != nil {
			return nil, err
		}
		if err := acc.SetPendingNonce(actionCtx.Nonce + 1); err != nil {
			return nil, errors.Wrap(err, "failed to set nonce")
		}
		if err := accountutil.StoreAccount(sm, actionCtx.Caller, acc); err != nil {
			return nil, errors.Wrap(err, "failed to update nonce")
		}
	}
	r := action.Receipt{
		Status:            status,
		BlockHeight:       blkCtx.BlockHeight,
		ActionHash:        actionCtx.ActionHash,
		GasConsumed:       gasConsumed,
		ContractAddress:   p.addr.String(),
		EffectiveGasPrice: protocol.EffectiveGasPrice(ctx, act),
	}
	r.AddLogs(logs...).AddTransactionLogs(depositLog...).AddTransactionLogs(tLogs...)
	return &r, nil
}

func (p *Protocol) skipContractStakingView(height uint64) bool {
	return height >= p.config.SkipContractStakingViewHeight
}

func (p *Protocol) needToReadCandsMap(ctx context.Context, height uint64) bool {
	fCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	return height > p.config.PersistStakingPatchBlock && fCtx.CandCenterHasAlias(height)
}

func (p *Protocol) needToWriteCandsMap(ctx context.Context, height uint64) bool {
	fCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	return height >= p.config.PersistStakingPatchBlock && fCtx.CandCenterHasAlias(height)
}

func (p *Protocol) contractStakingVotesFromIndexer(ctx context.Context, candidate address.Address, height uint64) (*big.Int, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	votes := big.NewInt(0)
	indexers := []ContractStakingIndexer{}
	if p.contractStakingIndexer != nil && featureCtx.AddContractStakingVotes {
		indexers = append(indexers, p.contractStakingIndexer)
	}
	if p.contractStakingIndexerV2 != nil && !featureCtx.LimitedStakingContract {
		indexers = append(indexers, p.contractStakingIndexerV2)
	}
	if p.contractStakingIndexerV3 != nil && featureCtx.TimestampedStakingContract {
		indexers = append(indexers, p.contractStakingIndexerV3)
	}

	for _, indexer := range indexers {
		btks, err := indexer.BucketsByCandidate(candidate, height)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get BucketsByCandidate from contractStakingIndexer")
		}
		for _, b := range btks {
			votes.Add(votes, p.contractBucketVotes(featureCtx, b))
		}
	}
	return votes, nil
}

func (p *Protocol) contractStakingVotesFromView(ctx context.Context, candidate address.Address, view *viewData) (*big.Int, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	votes := big.NewInt(0)
	views := []ContractStakeView{}
	if p.contractStakingIndexer != nil && featureCtx.AddContractStakingVotes {
		views = append(views, view.contractsStake.v1)
	}
	if p.contractStakingIndexerV2 != nil && !featureCtx.LimitedStakingContract {
		views = append(views, view.contractsStake.v2)
	}
	if p.contractStakingIndexerV3 != nil && featureCtx.TimestampedStakingContract {
		views = append(views, view.contractsStake.v3)
	}
	for _, cv := range views {
		v := cv.CandidateStakeVotes(ctx, candidate)
		if v == nil {
			continue
		}
		votes.Add(votes, v)
	}
	return votes, nil
}

func (p *Protocol) contractBucketVotes(fCtx protocol.FeatureCtx, bkt *VoteBucket) *big.Int {
	votes := big.NewInt(0)
	if bkt.isUnstaked() {
		return votes
	}
	if fCtx.FixContractStakingWeightedVotes {
		votes.Add(votes, p.calculateVoteWeight(bkt, false))
	} else {
		votes.Add(votes, bkt.StakedAmount)
	}
	return votes
}

func readCandCenterStateFromStateDB(sr protocol.StateReader) (CandidateList, CandidateList, CandidateList, error) {
	var (
		name, operator, owner CandidateList
	)
	if _, err := sr.State(&name, protocol.NamespaceOption(CandsMapNS), protocol.KeyOption(_nameKey)); err != nil {
		return nil, nil, nil, err
	}
	if _, err := sr.State(&operator, protocol.NamespaceOption(CandsMapNS), protocol.KeyOption(_operatorKey)); err != nil {
		return nil, nil, nil, err
	}
	if _, err := sr.State(&owner, protocol.NamespaceOption(CandsMapNS), protocol.KeyOption(_ownerKey)); err != nil {
		return nil, nil, nil, err
	}
	return name, operator, owner, nil
}

func contractStakingIndexerAt(index ContractStakingIndexer, sr protocol.StateReader, delay bool) (ContractStakingIndexer, error) {
	if index == nil {
		return nil, nil
	}
	srHeight, err := sr.Height()
	if err != nil {
		return nil, err
	}
	if delay {
		srHeight--
	}
	indexHeight, err := index.Height()
	if err != nil {
		return nil, err
	}
	if index.StartHeight() > srHeight || indexHeight == srHeight {
		return index, nil
	} else if indexHeight < srHeight {
		return nil, errors.Errorf("indexer height %d is too old for state reader height %d", indexHeight, srHeight)
	}
	return index.IndexerAt(sr), nil
}
