// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
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
	// TODO: it works only for one instance per protocol definition now
	_protocolID  = "rewarding"
	_v2Namespace = "Rewarding"
)

var (
	_adminKey                    = []byte("adm")
	_fundKey                     = []byte("fnd")
	_blockRewardHistoryKeyPrefix = []byte("brh")
	_epochRewardHistoryKeyPrefix = []byte("erh")
	_accountKeyPrefix            = []byte("acc")
	_exemptKey                   = []byte("xpt")
	errInvalidEpoch              = errors.New("invalid start/end epoch number")
)

// Protocol defines the protocol of the rewarding fund and the rewarding process. It allows the admin to config the
// reward amount, users to donate tokens to the fund, block producers to grant them block and epoch reward and,
// beneficiaries to claim the balance into their personal account.
type Protocol struct {
	keyPrefix []byte
	addr      address.Address
	cfg       genesis.Rewarding
}

// NewProtocol instantiates a rewarding protocol instance.
func NewProtocol(cfg genesis.Rewarding) *Protocol {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of rewarding protocol", zap.Error(err))
	}
	if err = validateFoundationBonusExtension(cfg); err != nil {
		log.L().Panic("failed to validate foundation bonus extension", zap.Error(err))
	}
	return &Protocol{
		keyPrefix: h[:],
		addr:      addr,
		cfg:       cfg,
	}
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
	switch blkCtx.BlockHeight {
	case g.AleutianBlockHeight:
		return p.SetReward(ctx, sm, g.AleutianEpochReward(), false)
	case g.DardanellesBlockHeight:
		return p.SetReward(ctx, sm, g.DardanellesBlockReward(), true)
	case g.GreenlandBlockHeight:
		return p.migrateValueGreenland(ctx, sm)
	case g.KamchatkaBlockHeight:
		return p.setFoundationBonusExtension(ctx, sm)
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
	return p.deleteStateV1(sm, key)
}

func (p *Protocol) setFoundationBonusExtension(ctx context.Context, sm protocol.StateManager) error {
	a := admin{}
	if _, err := p.state(ctx, sm, _adminKey, &a); err != nil {
		return err
	}

	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	blkCtx := protocol.MustGetBlockCtx(ctx)
	newLastEpoch := rp.GetEpochNum(blkCtx.BlockHeight) + 8760

	if a.foundationBonusLastEpoch < p.cfg.FoundationBonusP2EndEpoch {
		a.foundationBonusLastEpoch = p.cfg.FoundationBonusP2EndEpoch
	}
	if a.foundationBonusLastEpoch < newLastEpoch {
		a.foundationBonusLastEpoch = newLastEpoch
	}
	return p.putState(ctx, sm, _adminKey, &a)
}

// CreatePostSystemActions creates a list of system actions to be appended to block actions
func (p *Protocol) CreatePostSystemActions(ctx context.Context, _ protocol.StateReader) ([]action.Envelope, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	grants := []action.Envelope{createGrantRewardAction(action.BlockReward, blkCtx.BlockHeight)}
	rp := rolldpos.FindProtocol(protocol.MustGetRegistry(ctx))
	if rp != nil && blkCtx.BlockHeight == rp.GetEpochLastBlockHeight(rp.GetEpochNum(blkCtx.BlockHeight)) {
		grants = append(grants, createGrantRewardAction(action.EpochReward, blkCtx.BlockHeight))
	}

	return grants, nil
}

func createGrantRewardAction(rewardType int, height uint64) action.Envelope {
	builder := action.EnvelopeBuilder{}
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(rewardType).SetHeight(height).Build()

	return builder.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
}

// Validate validates a reward action
func (p *Protocol) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	if !protocol.MustGetFeatureCtx(ctx).ValidateRewardProtocol {
		return nil
	}
	switch act.(type) {
	case *action.GrantReward:
		actionCtx := protocol.MustGetActionCtx(ctx)
		if !address.Equal(protocol.MustGetBlockCtx(ctx).Producer, actionCtx.Caller) {
			return errors.New("Only producer could create reward")
		}
		if actionCtx.GasPrice != nil && actionCtx.GasPrice.Cmp(big.NewInt(0)) != 0 || actionCtx.IntrinsicGas != 0 {
			return errors.New("invalid gas price or intrinsic gas for reward action")
		}
	}
	return nil
}

// Handle handles the actions on the rewarding protocol
func (p *Protocol) Handle(
	ctx context.Context,
	act action.Action,
	sm protocol.StateManager,
) (*action.Receipt, error) {
	// TODO: simplify the boilerplate
	switch act := act.(type) {
	case *action.DepositToRewardingFund:
		si := sm.Snapshot()
		rlog, err := p.Deposit(ctx, sm, act.Amount(), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		if err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleUserAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		return p.settleUserAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Success), si, nil, rlog)
	case *action.ClaimFromRewardingFund:
		si := sm.Snapshot()
		rlog, err := p.Claim(ctx, sm, act.Amount())
		if err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleUserAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
		}
		return p.settleUserAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Success), si, nil, rlog)
	case *action.GrantReward:
		switch act.RewardType() {
		case action.BlockReward:
			si := sm.Snapshot()
			rewardLog, err := p.GrantBlockReward(ctx, sm)
			if err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleSystemAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
			}
			if rewardLog == nil {
				return p.settleSystemAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Success), si, nil)
			}
			return p.settleSystemAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Success), si, []*action.Log{rewardLog})
		case action.EpochReward:
			si := sm.Snapshot()
			rewardLogs, err := p.GrantEpochReward(ctx, sm)
			if err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleSystemAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Failure), si, nil)
			}
			return p.settleSystemAction(ctx, sm, uint64(iotextypes.ReceiptStatus_Success), si, rewardLogs)
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
	default:
		return nil, uint64(0), errors.New("corresponding method isn't found")
	}
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
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	return sm.State(value, protocol.LegacyKeyOption(keyHash))
}

func (p *Protocol) stateV2(sm protocol.StateReader, key []byte, value interface{}) (uint64, error) {
	k := append(p.keyPrefix, key...)
	return sm.State(value, protocol.KeyOption(k), protocol.NamespaceOption(_v2Namespace))
}

func (p *Protocol) putState(ctx context.Context, sm protocol.StateManager, key []byte, value interface{}) error {
	if useV2Storage(ctx) {
		return p.putStateV2(sm, key, value)
	}
	return p.putStateV1(sm, key, value)
}

func (p *Protocol) putStateV1(sm protocol.StateManager, key []byte, value interface{}) error {
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	_, err := sm.PutState(value, protocol.LegacyKeyOption(keyHash))
	return err
}

func (p *Protocol) putStateV2(sm protocol.StateManager, key []byte, value interface{}) error {
	k := append(p.keyPrefix, key...)
	_, err := sm.PutState(value, protocol.KeyOption(k), protocol.NamespaceOption(_v2Namespace))
	return err
}

func (p *Protocol) deleteState(ctx context.Context, sm protocol.StateManager, key []byte) error {
	if useV2Storage(ctx) {
		return p.deleteStateV2(sm, key)
	}
	return p.deleteStateV1(sm, key)
}

func (p *Protocol) deleteStateV1(sm protocol.StateManager, key []byte) error {
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	_, err := sm.DelState(protocol.LegacyKeyOption(keyHash))
	if errors.Cause(err) == state.ErrStateNotExist {
		// don't care if not exist
		return nil
	}
	return err
}

func (p *Protocol) deleteStateV2(sm protocol.StateManager, key []byte) error {
	k := append(p.keyPrefix, key...)
	_, err := sm.DelState(protocol.KeyOption(k), protocol.NamespaceOption(_v2Namespace))
	if errors.Cause(err) == state.ErrStateNotExist {
		// don't care if not exist
		return nil
	}
	return err
}

func (p *Protocol) settleSystemAction(
	ctx context.Context,
	sm protocol.StateManager,
	status uint64,
	si int,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) (*action.Receipt, error) {
	return p.settleAction(ctx, sm, status, si, true, logs, tLogs...)
}

func (p *Protocol) settleUserAction(
	ctx context.Context,
	sm protocol.StateManager,
	status uint64,
	si int,
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) (*action.Receipt, error) {
	return p.settleAction(ctx, sm, status, si, false, logs, tLogs...)
}

func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
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
	if !isSystemAction || !protocol.MustGetFeatureCtx(ctx).SkipUpdateForSystemAction {
		gasFee := big.NewInt(0).Mul(actionCtx.GasPrice, big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
		depositLog, err := DepositGas(ctx, sm, gasFee)
		if err != nil {
			return nil, err
		}
		if depositLog != nil {
			tLogs = append(tLogs, depositLog)
		}
		if err := p.increaseNonce(sm, actionCtx.Caller, actionCtx.Nonce, isSystemAction); err != nil {
			return nil, err
		}
	}
	return p.createReceipt(status, blkCtx.BlockHeight, actionCtx.ActionHash, actionCtx.IntrinsicGas, logs, tLogs...), nil
}

func (p *Protocol) increaseNonce(sm protocol.StateManager, addr address.Address, nonce uint64, isSystemAction bool) error {
	acc, err := accountutil.LoadOrCreateAccount(sm, addr)
	if err != nil {
		return err
	}
	if !isSystemAction || nonce != 0 {
		if err := acc.SetNonce(nonce); err != nil {
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
	logs []*action.Log,
	tLogs ...*action.TransactionLog,
) *action.Receipt {
	// TODO: need to review the fields
	return (&action.Receipt{
		Status:          status,
		BlockHeight:     blkHeight,
		ActionHash:      actHash,
		GasConsumed:     gasConsumed,
		ContractAddress: p.addr.String(),
	}).AddLogs(logs...).AddTransactionLogs(tLogs...)
}
