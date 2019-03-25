// Copyright (c) 2019 IoTeX
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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// ProtocolID is the protocol ID
	// TODO: it works only for one instance per protocol definition now
	ProtocolID = "rewarding"
)

var (
	adminKey                    = []byte("adm")
	fundKey                     = []byte("fnd")
	blockRewardHistoryKeyPrefix = []byte("brh")
	epochRewardHistoryKeyPrefix = []byte("erh")
	accountKeyPrefix            = []byte("acc")
	exemptKey                   = []byte("xpt")
)

// Protocol defines the protocol of the rewarding fund and the rewarding process. It allows the admin to config the
// reward amount, users to donate tokens to the fund, block producers to grant them block and epoch reward and,
// beneficiaries to claim the balance into their personal account.
type Protocol struct {
	cm        protocol.ChainManager
	keyPrefix []byte
	addr      address.Address
	rp        *rolldpos.Protocol
}

// NewProtocol instantiates a rewarding protocol instance.
func NewProtocol(cm protocol.ChainManager, rp *rolldpos.Protocol) *Protocol {
	h := hash.Hash160b([]byte(ProtocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of rewarding protocol", zap.Error(err))
	}
	return &Protocol{
		cm:        cm,
		keyPrefix: h[:],
		addr:      addr,
		rp:        rp,
	}
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
		if err := p.Deposit(ctx, sm, act.Amount()); err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleAction(ctx, sm, action.FailureReceiptStatus, si)
		}
		return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si)
	case *action.ClaimFromRewardingFund:
		si := sm.Snapshot()
		if err := p.Claim(ctx, sm, act.Amount()); err != nil {
			log.L().Debug("Error when handling rewarding action", zap.Error(err))
			return p.settleAction(ctx, sm, action.FailureReceiptStatus, si)
		}
		return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si)
	case *action.GrantReward:
		switch act.RewardType() {
		case action.BlockReward:
			si := sm.Snapshot()
			if err := p.GrantBlockReward(ctx, sm); err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleAction(ctx, sm, action.FailureReceiptStatus, si)
			}
			return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si)
		case action.EpochReward:
			si := sm.Snapshot()
			if err := p.GrantEpochReward(ctx, sm); err != nil {
				log.L().Debug("Error when handling rewarding action", zap.Error(err))
				return p.settleAction(ctx, sm, action.FailureReceiptStatus, si)
			}
			return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si)
		}
	}
	return nil, nil
}

// Validate validates the actions on the rewarding protocol
func (p *Protocol) Validate(
	ctx context.Context,
	act action.Action,
) error {
	// TODO: validate interface shouldn't be required for protocol code
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(
	ctx context.Context,
	sm protocol.StateManager,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	switch string(method) {
	case "UnclaimedBalance":
		if len(args) != 1 {
			return nil, errors.Errorf("invalid number of arguments %d", len(args))
		}
		addr, err := address.FromString(string(args[0]))
		if err != nil {
			return nil, err
		}
		balance, err := p.UnclaimedBalance(ctx, sm, addr)
		if err != nil {
			return nil, err
		}
		return []byte(balance.String()), nil
	default:
		return nil, errors.New("corresponding method isn't found")
	}
}

func (p *Protocol) state(sm protocol.StateManager, key []byte, value interface{}) error {
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	return sm.State(keyHash, value)
}

func (p *Protocol) putState(sm protocol.StateManager, key []byte, value interface{}) error {
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	return sm.PutState(keyHash, value)
}

func (p *Protocol) deleteState(sm protocol.StateManager, key []byte) error {
	keyHash := hash.Hash160b(append(p.keyPrefix, key...))
	return sm.DelState(keyHash)
}

func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	status uint64,
	si int,
) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if status == action.FailureReceiptStatus {
		if err := sm.Revert(si); err != nil {
			return nil, err
		}
	}
	gasFee := big.NewInt(0).Mul(raCtx.GasPrice, big.NewInt(0).SetUint64(raCtx.IntrinsicGas))
	if err := DepositGas(ctx, sm, gasFee, raCtx.Registry); err != nil {
		return nil, err
	}
	if err := p.increaseNonce(sm, raCtx.Caller, raCtx.Nonce); err != nil {
		return nil, err
	}
	return p.createReceipt(status, raCtx.ActionHash, raCtx.IntrinsicGas), nil
}

func (p *Protocol) increaseNonce(sm protocol.StateManager, addr address.Address, nonce uint64) error {
	acc, err := accountutil.LoadOrCreateAccount(sm, addr.String(), big.NewInt(0))
	if err != nil {
		return err
	}
	// TODO: this check shouldn't be necessary
	if nonce > acc.Nonce {
		acc.Nonce = nonce
	}
	return accountutil.StoreAccount(sm, addr.String(), acc)
}

func (p *Protocol) createReceipt(status uint64, actHash hash.Hash256, gasConsumed uint64) *action.Receipt {
	// TODO: need to review the fields
	return &action.Receipt{
		ReturnValue:     nil,
		Status:          status,
		ActHash:         actHash,
		GasConsumed:     gasConsumed,
		ContractAddress: p.addr.String(),
		Logs:            nil,
	}
}
