// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/enc"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const (
	// TODO: it works only for one instance per protocol definition now
	_protocolID           = "rewarding"
	_v2RewardingNamespace = state.RewardingNamespace
)

var (
	_adminKey                    = []byte("adm")
	_fundKey                     = []byte("fnd")
	_blockRewardHistoryKeyPrefix = state.BlockRewardHistoryKeyPrefix
	_epochRewardHistoryKeyPrefix = state.EpochRewardHistoryKeyPrefix
	_accountKeyPrefix            = []byte("acc")
	_exemptKey                   = []byte("xpt")
	// _pendingBlockRewardPoolKeyPrefix keys the per-delegate IIP-59 block
	// reward pool. Full key layout is "pbrp/" || candidate identifier bytes,
	// with entries deleted at drain (see pending_block_reward.go).
	_pendingBlockRewardPoolKeyPrefix = []byte("pbrp/")
	errInvalidEpoch                  = errors.New("invalid start/end epoch number")
)

// Protocol defines the protocol of the rewarding fund and the rewarding process. It allows the admin to config the
// reward amount, users to donate tokens to the fund, block producers to grant them block and epoch reward and,
// beneficiaries to claim the balance into their personal account.
type Protocol struct {
	keyPrefix []byte
	addr      address.Address
	cfg       genesis.Rewarding
	// autoDepositBridge is the IIP-59 AutoDeposit contract adapter used by
	// distributeVoterReward for per-voter compound routing. Nil means the
	// network has no AutoDeposit contract configured: every voter share is
	// credited directly to the primary account and no compound
	// deposit is attempted.
	autoDepositBridge *autodeposit.Bridge
	// autoDepositBucketReaderFactory lets tests inject a fake BucketReader
	// without pulling the EVM state adapter into scope. Nil in production;
	// production paths construct SlotBucketReader via
	// resolveAutoDepositBucketReader — one adapter setup per drain,
	// direct-slot reads per voter (see docs/iip-59-perf-report.md).
	autoDepositBucketReaderFactory func(autodeposit.SlotReader) autodeposit.BucketReader
}

// Option customises a rewarding Protocol at construction. Options are
// applied in order after the default fields are set.
type Option func(*Protocol)

// WithAutoDepositBridge wires the IIP-59 AutoDeposit contract bridge into
// the rewarding protocol. Pass nil (or omit the option) when the network
// has no AutoDeposit contract; distributeVoterReward will then credit
// every voter share instead of routing to compound.
func WithAutoDepositBridge(bridge *autodeposit.Bridge) Option {
	return func(p *Protocol) { p.autoDepositBridge = bridge }
}

// WithAutoDepositBucketReader injects a factory for the BucketReader used
// against the AutoDeposit contract. Intended for tests that supply a fake
// in place of the direct-slot reader; production leaves this unset so
// Bridge.NewSlotBucketReader is used.
func WithAutoDepositBucketReader(factory func(autodeposit.SlotReader) autodeposit.BucketReader) Option {
	return func(p *Protocol) { p.autoDepositBucketReaderFactory = factory }
}

// NewProtocol instantiates a rewarding protocol instance.
func NewProtocol(cfg genesis.Rewarding, opts ...Option) *Protocol {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of rewarding protocol", zap.Error(err))
	}
	if err = validateFoundationBonusExtension(cfg); err != nil {
		log.L().Panic("failed to validate foundation bonus extension", zap.Error(err))
	}
	p := &Protocol{
		keyPrefix: state.RewardingKeyPrefix[:],
		addr:      addr,
		cfg:       cfg,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ProtocolAddr returns the address generated from protocol id
func ProtocolAddr() address.Address {
	return protocol.HashStringToAddress(_protocolID)
}

// verify that foundation bonus extension epochs are in increasing order
func validateFoundationBonusExtension(cfg genesis.Rewarding) error {
	if cfg.FoundationBonusP2StartEpoch > 0 || cfg.FoundationBonusP2EndEpoch > 0 {
		if cfg.FoundationBonusP2StartEpoch < cfg.FoundationBonusLastEpoch || cfg.FoundationBonusP2EndEpoch < cfg.FoundationBonusP2StartEpoch {
			return errInvalidEpoch
		}
	}
	return nil
}

// FindProtocol finds the registered protocol from registry
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
		log.S().Panic("fail to cast reward protocol")
	}
	return rp
}

// CreatePreStates updates state manager
func (p *Protocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	g := genesis.MustExtractGenesisContext(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	// set current block reward not granted for erigon db
	var indexBytes [8]byte
	enc.MachineEndian.PutUint64(indexBytes[:], blkCtx.BlockHeight)
	err := p.deleteState(ctx, sm, append(_blockRewardHistoryKeyPrefix, indexBytes[:]...), &rewardHistory{}, protocol.ErigonStoreOnlyOption())
	if err != nil && !errors.Is(err, state.ErrErigonStoreNotSupported) {
		return errors.Wrap(err, "failed to delete block reward history for erigon store")
	}
	if err := p.collectEraCOWGarbage(ctx, sm); err != nil {
		return err
	}
	switch blkCtx.BlockHeight {
	case g.AleutianBlockHeight:
		return p.SetReward(ctx, sm, g.AleutianEpochReward(), false)
	case g.DardanellesBlockHeight:
		return p.SetReward(ctx, sm, g.DardanellesBlockReward(), true)
	case g.GreenlandBlockHeight:
		return p.migrateValueGreenland(ctx, sm)
	case g.KamchatkaBlockHeight:
		return p.setFoundationBonusExtension(ctx, sm)
	case g.WakeBlockHeight:
		return p.SetReward(ctx, sm, g.WakeBlockReward(), true)
	}
	return nil
}

// _eraCOWGarbagePerBlock is how many sealed era copy-on-write entries are
// deleted per block. An era with heavy bucket churn can leave tens of
// thousands behind; deleting them in one block would spend the very block
// budget the drain itself is chunked to protect, and there is no deadline —
// the entries are unreachable the moment the era is sealed, so the backlog
// only has to drain before the next era accumulates a comparable one.
const _eraCOWGarbagePerBlock = 256

// collectEraCOWGarbage deletes a bounded batch of copies left by sealed IIP-59
// era windows. It runs from CreatePreStates so it happens once per block on
// every node in the same place, before any action executes.
//
// No-op pre-activation and whenever the backlog is empty; the staking side
// checks the fork gate before touching state.
func (p *Protocol) collectEraCOWGarbage(ctx context.Context, sm protocol.StateManager) error {
	n, err := staking.CollectEraCOWGarbage(ctx, sm, _eraCOWGarbagePerBlock)
	if err != nil {
		return errors.Wrap(err, "failed to collect era copy-on-write garbage")
	}
	if n > 0 {
		addIIP59Items("eracow_gc", n)
	}
	return nil
}

func (p *Protocol) migrateValueGreenland(_ context.Context, sm protocol.StateManager) error {
	if err := p.migrateValue(sm, _adminKey, &admin{}); err != nil {
		return err
	}
	if err := p.migrateValue(sm, _fundKey, &fund{}); err != nil {
		return err
	}
	return p.migrateValue(sm, _exemptKey, &exempt{})
}

func (p *Protocol) migrateValue(sm protocol.StateManager, key []byte, value interface{}) error {
	if _, err := p.stateV1(sm, key, value); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			// doesn't exist now just skip migration
			return nil
		}
		return err
	}
	if err := p.putStateV2(sm, key, value); err != nil {
		return err
	}
	return p.deleteStateV1(sm, key, value)
}

// _foundationBonusExtensionEpochs is how far past the activation block the
// Kamchatka foundation-bonus extension runs — one year of hourly epochs.
const _foundationBonusExtensionEpochs uint64 = 8760

func (p *Protocol) setFoundationBonusExtension(ctx context.Context, sm protocol.StateManager) error {
	a := admin{}
	if _, err := p.state(ctx, sm, _adminKey, &a); err != nil {
		return err
	}

	rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx))
	if rp == nil {
		return nil
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	newLastEpoch := rp.GetEpochNum(blkCtx.BlockHeight) + _foundationBonusExtensionEpochs

	if a.foundationBonusLastEpoch < p.cfg.FoundationBonusP2EndEpoch {
		a.foundationBonusLastEpoch = p.cfg.FoundationBonusP2EndEpoch
	}
	if a.foundationBonusLastEpoch < newLastEpoch {
		a.foundationBonusLastEpoch = newLastEpoch
	}
	return p.putState(ctx, sm, _adminKey, &a)
}

// CreatePostSystemActions creates a list of system actions to be appended to block actions
func (p *Protocol) CreatePostSystemActions(ctx context.Context, sr protocol.StateReader) ([]action.Envelope, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	grants := []action.Envelope{createGrantRewardAction(action.BlockReward, blkCtx.BlockHeight)}
	rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx))
	epochLast := rp != nil && blkCtx.BlockHeight == rp.GetEpochLastBlockHeight(rp.GetEpochNum(blkCtx.BlockHeight))
	if epochLast {
		grants = append(grants, createGrantRewardAction(action.EpochReward, blkCtx.BlockHeight))
		return grants, nil
	}
	// IIP-59 continuation dispatch: an incomplete chunked drain emits a dedicated
	// VoterRewardChunk grant on every non-epoch-boundary block until
	// GrantVoterRewardChunk's coda marks the cursor complete. Cursor is only
	// ever written on the fork-on path, so pre-fork blocks skip the
	// read entirely (state is empty).
	if !protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		cursor, err := p.readEpochDrainCursor(ctx, sr)
		if err != nil {
			return nil, err
		}
		if cursor != nil && !cursor.Completed {
			grants = append(grants, createGrantRewardAction(action.VoterRewardChunk, blkCtx.BlockHeight))
		}
	}
	return grants, nil
}

func createGrantRewardAction(rewardType int, height uint64) action.Envelope {
	builder := action.EnvelopeBuilder{}
	grant := action.NewGrantReward(rewardType, height)

	return builder.SetNonce(0).SetGasPrice(big.NewInt(0)).
		SetAction(grant).Build()
}

// Validate validates a reward action
func (p *Protocol) Validate(ctx context.Context, elp action.Envelope, sr protocol.StateReader) error {
	switch act := elp.Action().(type) {
	case *action.GrantReward:
		actionCtx := protocol.MustGetActionCtx(ctx)
		if !address.Equal(protocol.MustGetBlockCtx(ctx).Producer, actionCtx.Caller) {
			return errors.New("Only producer could create reward")
		}
		if actionCtx.GasPrice != nil && actionCtx.GasPrice.Cmp(big.NewInt(0)) != 0 || actionCtx.IntrinsicGas != 0 {
			return errors.New("invalid gas price or intrinsic gas for reward action")
		}
		// VoterRewardChunk is an IIP-59 action; pre-fork blocks must
		// never accept one even if a producer crafts it manually.
		if act.RewardType() == action.VoterRewardChunk && protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
			return errors.New("voter reward chunk action not enabled yet")
		}
	case *action.ClaimFromRewardingFund:
		if !protocol.MustGetFeatureCtx(ctx).AddClaimRewardAddress && act.Address() != nil {
			return errors.New("claim reward address not enabled yet")
		}
	case *action.SetVoterRewardDestination:
		if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
			return errors.New("voter reward destination is not enabled yet")
		}
		return act.SanityCheck()
	}
	return nil
}

// Handle handles the actions on the rewarding protocol
func (p *Protocol) Handle(
	ctx context.Context,
	elp action.Envelope,
	sm protocol.StateManager,
) (receipt *action.Receipt, err error) {
	// TODO: simplify the boilerplate
	var (
		si  = sm.Snapshot()
		act = elp.Action()
	)
	switch act := act.(type) {
	case *action.DepositToRewardingFund:
		rlog, err := p.Deposit(ctx, sm, act.Amount(), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		if err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, nil, rlog...)
	case *action.ClaimFromRewardingFund:
		addr := protocol.MustGetActionCtx(ctx).Caller
		if act.Address() != nil {
			addr = act.Address()
		}
		rlog, err := p.Claim(ctx, sm, act.ClaimAmount(), addr)
		if err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, nil, rlog)
	case *action.SetVoterRewardDestination:
		actCtx := protocol.MustGetActionCtx(ctx)
		oldRecipient, newRecipient, err := p.setVoterRewardDestination(ctx, sm, actCtx.Caller, act.Recipient())
		if err != nil {
			log.L().Debug("Error when setting voter reward destination", zap.Error(err))
			return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		topics, data, err := action.PackVoterRewardDestinationSetEvent(actCtx.Caller, oldRecipient, newRecipient)
		if err != nil {
			log.L().Debug("Error when encoding voter reward destination event", zap.Error(err))
			return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		blkCtx := protocol.MustGetBlockCtx(ctx)
		return p.settleUserAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, []*action.Log{{
			Address:     p.addr.String(),
			Topics:      topics,
			Data:        data,
			BlockHeight: blkCtx.BlockHeight,
			ActionHash:  actCtx.ActionHash,
		}})
	case *action.GrantReward:
		switch act.RewardType() {
		case action.BlockReward:
			rewardLog, transactionLogs, err := p.GrantBlockReward(ctx, sm)
			if err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
			}
			if rewardLog == nil {
				return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, nil, transactionLogs...)
			}
			return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, []*action.Log{rewardLog}, transactionLogs...)
		case action.EpochReward:
			transactionLogs, rewardLogs, err := p.GrantEpochReward(ctx, sm)
			if err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
			}
			return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, rewardLogs, transactionLogs...)
		case action.VoterRewardChunk:
			transactionLogs, rewardLogs, err := p.GrantVoterRewardChunk(ctx, sm)
			if err != nil {
				// Deliberately louder than its siblings above. A failed block or
				// epoch grant is self-announcing -- the receipt is the whole
				// story and the next block starts over. A failed drain chunk is
				// not: the cursor stays put, the chain keeps advancing, and the
				// era's remaining voter payouts are silently dropped when the
				// next boundary rewrites the cursor.
				p.reportVoterRewardChunkFailure(ctx, sm, err)
				if !voterChunkErrorIsSettleable(err) {
					// Not a verdict every node reaches identically -- most of
					// what the drain can raise is a state read, a state write,
					// or a range scan, and a working set's ability to serve an
					// ordered range scan is a node-local capability, not chain
					// state. Settling a Failure receipt here would let the
					// proposer commit "no payouts, cursor unmoved" while
					// validators that could serve the scan commit the payouts:
					// same block, two state roots. Propagate instead and let
					// the block fail. See voterChunkSettleableError for why the
					// default is to halt and what may opt out.
					return nil, err
				}
				// Explicitly marked as derivable from committed state (see
				// settleableVoterChunkError): every node agrees, so a Failure
				// receipt is a consistent outcome and the block still commits.
				return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
			}
			return p.settleSystemAction(ctx, sm, elp, uint64(iotextypes.ReceiptStatus_Success), si, rewardLogs, transactionLogs...)
		}
	}
	return nil, nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(
	ctx context.Context,
	sr protocol.StateReader,
	method []byte,
	args ...[]byte,
) ([]byte, uint64, error) {
	switch string(method) {
	case "AvailableBalance":
		balance, height, err := p.AvailableBalance(ctx, sr)
		if err != nil {
			return nil, uint64(0), err
		}
		return []byte(balance.String()), height, nil
	case "TotalBalance":
		balance, height, err := p.TotalBalance(ctx, sr)
		if err != nil {
			return nil, uint64(0), err
		}
		return []byte(balance.String()), height, nil
	case "UnclaimedBalance":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		addr, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		balance, height, err := p.UnclaimedBalance(ctx, sr, addr)
		if err != nil {
			return nil, uint64(0), err
		}
		return []byte(balance.String()), height, nil
	case "PendingBlockRewardPool":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		candID, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		balance, err := p.readPendingBlockRewardPool(ctx, sr, candID.Bytes())
		if err != nil {
			return nil, uint64(0), err
		}
		height, err := sr.Height()
		if err != nil {
			return nil, uint64(0), err
		}
		return []byte(balance.String()), height, nil
	case "PendingBlockRewardPoolIndex":
		if len(args) != 0 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		ids, err := p.readPendingBlockRewardPoolIndex(ctx, sr)
		if err != nil {
			return nil, uint64(0), err
		}
		return marshalWithHeight(sr, &rewardingpb.PendingBlockRewardPoolIndex{CandidateIdentifiers: ids})
	case "EpochDrainCursor":
		if len(args) != 0 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		cursor, err := p.readEpochDrainCursor(ctx, sr)
		if err != nil {
			return nil, uint64(0), err
		}
		if cursor == nil {
			return marshalWithHeight(sr, &rewardingpb.EpochDrainCursor{})
		}
		data, err := cursor.Serialize()
		if err != nil {
			return nil, uint64(0), err
		}
		return bytesWithHeight(sr, data)
	case "VoterRewardSnapshot":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		candID, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		snapshot, err := staking.PollSnapshotFor(sr, candID)
		if err != nil {
			return nil, uint64(0), err
		}
		// Scalars only. The frozen (voter, weight) list this used to enumerate
		// no longer exists; a caller that wants a voter's position and amount
		// asks VoterRewardStatus, which is per-voter and answers across every
		// delegate the voter has a frozen bucket with.
		return marshalWithHeight(sr, &stakingpb.CandidatePollSnapshot{
			BlockCommissionBasisPoints: snapshot.BlockCommissionBasisPoints,
			EpochCommissionBasisPoints: snapshot.EpochCommissionBasisPoints,
			Registered:                 snapshot.Registered,
			OnchainRewardEnabled:       snapshot.OnchainRewardEnabled,
			TotalWeight:                safeBig(snapshot.TotalWeight).Bytes(),
			SnapshotHash:               snapshot.SnapshotHash[:],
			FreezeHeight:               snapshot.FreezeHeight,
			SelfStakeBucketIdx:         snapshot.SelfStakeBucketIdx,
		})
	case "VoterRewardAddress":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		candID, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		routing, err := resolveDelegateRewardRouting(ctx, sr, candID)
		if err != nil {
			return nil, uint64(0), err
		}
		rewardAddr := routing.legacyRewardAddress
		if routing.onchainRewardEnabled {
			rewardAddr = routing.owner
		}
		return marshalWithHeight(sr, &rewardingpb.VoterRewardAddress{
			Address: rewardAddr.Bytes(), ExplicitlySet: routing.rewardAddressUpdated,
		})
	case "VoterRewardDestination":
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		voter, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		recipient, explicitlySet, updatedHeight, err := p.resolveVoterRewardDestination(ctx, sr, voter)
		if err != nil {
			return nil, uint64(0), err
		}
		return marshalWithHeight(sr, &rewardingpb.VoterRewardDestination{
			Recipient: recipient.Bytes(), ExplicitlySet: explicitlySet, UpdatedHeight: updatedHeight,
		})
	case "VoterRewardStatus":
		// One argument, not two: the drain pays a voter once for everything they
		// are owed across every delegate, so the answer is per-voter.
		if len(args) != 1 {
			return nil, uint64(0), errors.Errorf("invalid number of arguments %d", len(args))
		}
		voter, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, uint64(0), err
		}
		status, err := p.voterRewardStatus(ctx, sr, voter)
		if err != nil {
			return nil, uint64(0), err
		}
		return marshalWithHeight(sr, status)
	default:
		return nil, uint64(0), errors.New("corresponding method isn't found")
	}
}

// marshalWithHeight builds the (data, height, error) triple every ReadState
// case returns, stamping the response with the height it was read at.
func marshalWithHeight(sr protocol.StateReader, m proto.Message) ([]byte, uint64, error) {
	data, err := proto.Marshal(m)
	if err != nil {
		return nil, uint64(0), err
	}
	return bytesWithHeight(sr, data)
}

// bytesWithHeight is marshalWithHeight for a value that serializes itself.
func bytesWithHeight(sr protocol.StateReader, data []byte) ([]byte, uint64, error) {
	height, err := sr.Height()
	if err != nil {
		return nil, uint64(0), err
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

// useV2Storage return true after greenland when we start using v2 storage.
func useV2Storage(ctx context.Context) bool {
	return protocol.MustGetFeatureCtx(ctx).UseV2Storage
}

func (p *Protocol) state(ctx context.Context, sm protocol.StateReader, key []byte, value interface{}) (uint64, error) {
	h, _, err := p.stateCheckLegacy(ctx, sm, key, value)
	return h, err
}

func (p *Protocol) stateCheckLegacy(ctx context.Context, sm protocol.StateReader, key []byte, value interface{}) (uint64, bool, error) {
	if useV2Storage(ctx) {
		h, err := p.stateV2(sm, key, value)
		if errors.Cause(err) != state.ErrStateNotExist {
			return h, false, err
		}
	}
	h, err := p.stateV1(sm, key, value)
	return h, true, err
}

func (p *Protocol) stateV1(sm protocol.StateReader, key []byte, value interface{}) (uint64, error) {
	orgKey := append(p.keyPrefix, key...)
	keyHash := hash.Hash160b(orgKey)
	return sm.State(value, protocol.LegacyKeyOption(keyHash), protocol.ErigonStoreKeyOption(orgKey))
}

func (p *Protocol) stateV2(sm protocol.StateReader, key []byte, value interface{}) (uint64, error) {
	k := append(p.keyPrefix, key...)
	return sm.State(value, protocol.KeyOption(k), protocol.NamespaceOption(_v2RewardingNamespace))
}

func (p *Protocol) putState(ctx context.Context, sm protocol.StateManager, key []byte, value interface{}) error {
	if useV2Storage(ctx) {
		return p.putStateV2(sm, key, value)
	}
	return p.putStateV1(sm, key, value)
}

func (p *Protocol) putStateV1(sm protocol.StateManager, key []byte, value interface{}, opts ...protocol.StateOption) error {
	orgKey := append(p.keyPrefix, key...)
	keyHash := hash.Hash160b(orgKey)
	opts = append(opts, protocol.LegacyKeyOption(keyHash), protocol.ErigonStoreKeyOption(orgKey))
	_, err := sm.PutState(value, opts...)
	return err
}

func (p *Protocol) putStateV2(sm protocol.StateManager, key []byte, value interface{}) error {
	k := append(p.keyPrefix, key...)
	_, err := sm.PutState(value, protocol.KeyOption(k), protocol.NamespaceOption(_v2RewardingNamespace))
	return err
}

func (p *Protocol) deleteState(ctx context.Context, sm protocol.StateManager, key []byte, obj any, opts ...protocol.StateOption) error {
	if useV2Storage(ctx) {
		return p.deleteStateV2(sm, key, obj, opts...)
	}
	return p.deleteStateV1(sm, key, obj, opts...)
}

func (p *Protocol) deleteStateV1(sm protocol.StateManager, key []byte, obj any, opts ...protocol.StateOption) error {
	orgKey := append(p.keyPrefix, key...)
	keyHash := hash.Hash160b(orgKey)
	opt := append(opts, protocol.LegacyKeyOption(keyHash), protocol.ObjectOption(obj), protocol.ErigonStoreKeyOption(orgKey))
	_, err := sm.DelState(opt...)
	if errors.Cause(err) == state.ErrStateNotExist {
		// don't care if not exist
		return nil
	}
	return err
}

func (p *Protocol) deleteStateV2(sm protocol.StateManager, key []byte, value any, opts ...protocol.StateOption) error {
	k := append(p.keyPrefix, key...)
	opt := append(opts, protocol.KeyOption(k), protocol.ObjectOption(value), protocol.NamespaceOption(_v2RewardingNamespace))
	_, err := sm.DelState(opt...)
	if errors.Cause(err) == state.ErrStateNotExist {
		// don't care if not exist
		return nil
	}
	return err
}

func (p *Protocol) settleSystemAction(
	ctx context.Context,
	sm protocol.StateManager,
	act action.TxDynamicGas,
	status uint64,
	si int,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) (*action.Receipt, error) {
	return p.settleAction(ctx, sm, act, status, si, true, logs, tLogs...)
}

func (p *Protocol) settleUserAction(
	ctx context.Context,
	sm protocol.StateManager,
	act action.TxDynamicGas,
	status uint64,
	si int,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) (*action.Receipt, error) {
	return p.settleAction(ctx, sm, act, status, si, false, logs, tLogs...)
}

func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	act action.TxDynamicGas,
	status uint64,
	si int,
	isSystemAction bool,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if status == uint64(iotextypes.ReceiptStatus_Failure) {
		if err := sm.Revert(si); err != nil {
			return nil, err
		}
	}
	skipUpdateForSystemAction := protocol.MustGetFeatureCtx(ctx).FixGasAndNonceUpdate
	if !isSystemAction || !skipUpdateForSystemAction {
		priorityFee, baseFee, err := protocol.SplitGas(ctx, act, actionCtx.IntrinsicGas)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to split gas")
		}
		depositLog, err := DepositGas(ctx, sm, baseFee, protocol.PriorityFeeOption(priorityFee))
		if err != nil {
			return nil, err
		}
		if depositLog != nil {
			tLogs = append(tLogs, depositLog...)
		}
		if err := p.increaseNonce(
			ctx,
			sm,
			actionCtx.Caller,
			actionCtx.Nonce,
			!skipUpdateForSystemAction && actionCtx.Nonce == 0,
		); err != nil {
			return nil, err
		}
	}
	return p.createReceipt(status, blkCtx.BlockHeight, actionCtx.ActionHash, actionCtx.IntrinsicGas, protocol.EffectiveGasPrice(ctx, act), logs, tLogs...), nil
}

func (p *Protocol) increaseNonce(ctx context.Context, sm protocol.StateManager, addr address.Address, nonce uint64, skipSetNonce bool) error {
	accountCreationOpts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
	}
	acc, err := accountutil.LoadOrCreateAccount(sm, addr, accountCreationOpts...)
	if err != nil {
		return err
	}
	if !skipSetNonce {
		if err := acc.SetPendingNonce(nonce + 1); err != nil {
			return errors.Wrapf(err, "invalid nonce %d", nonce)
		}
	}
	return accountutil.StoreAccount(sm, addr, acc)
}

func (p *Protocol) createReceipt(
	status uint64,
	blkHeight uint64,
	actHash hash.Hash256,
	gasConsumed uint64,
	price *big.Int,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) *action.Receipt {
	// TODO: need to review the fields
	return (&action.Receipt{
		Status:            status,
		BlockHeight:       blkHeight,
		ActionHash:        actHash,
		GasConsumed:       gasConsumed,
		ContractAddress:   p.addr.String(),
		EffectiveGasPrice: price,
	}).AddLogs(logs...).AddTransactionLogs(tLogs...)
}
