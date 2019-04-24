// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"context"
	"math/big"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	// ProtocolID is the protocol ID
	// TODO: it works only for one instance per protocol definition now
	ProtocolID = "config"
)

// Protocol handles the on-chain config
type Protocol struct {
	initActiveProtocols []string
	keyPrefix           []byte
	addr                address.Address
	admin               address.Address
}

// NewProtocol instantiate a protocol struct
func NewProtocol(admin address.Address, initActiveProtocols []string) *Protocol {
	h := hash.Hash160b([]byte(ProtocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of rewarding protocol", zap.Error(err))
	}
	return &Protocol{
		initActiveProtocols: initActiveProtocols,
		keyPrefix:           h[:],
		addr:                addr,
		admin:               admin,
	}
}

// Handle handles the action
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.UpdateActiveProtocols:
		si := sm.Snapshot()
		uapLog, err := p.UpdateActiveProtocols(ctx, sm, act.Additions(), act.Removals())
		if err != nil {
			log.L().Debug("Error when handling UpdateActiveProtocols action", zap.Error(err))
			return p.settleAction(ctx, sm, action.FailureReceiptStatus, si)
		}
		if uapLog == nil {
			return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si)
		}
		return p.settleAction(ctx, sm, action.SuccessReceiptStatus, si, uapLog)
	}
	return nil, nil
}

// ReadState reads the state of this protocol
func (p *Protocol) ReadState(
	ctx context.Context,
	sm protocol.StateManager,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	switch string(method) {
	case "ActiveProtocols":
		aps, err := p.ActiveProtocols(ctx, sm)
		if err != nil {
			return nil, err
		}
		return []byte(strings.Join(aps, ",")), nil
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

func (p *Protocol) assertAdmin(addr address.Address) error {
	if address.Equal(addr, p.admin) {
		return nil
	}
	return errors.Errorf("%s is not the admin", addr.String())
}

// TODO: dedup the code against rewarding protocol
func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	status uint64,
	si int,
	logs ...*action.Log,
) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if status == action.FailureReceiptStatus {
		if err := sm.Revert(si); err != nil {
			return nil, err
		}
	}
	gasFee := big.NewInt(0).Mul(raCtx.GasPrice, big.NewInt(0).SetUint64(raCtx.IntrinsicGas))
	if err := rewarding.DepositGas(ctx, sm, gasFee, raCtx.Registry); err != nil {
		return nil, err
	}
	if err := p.increaseNonce(sm, raCtx.Caller, raCtx.Nonce); err != nil {
		return nil, err
	}
	return p.createReceipt(status, raCtx.BlockHeight, raCtx.ActionHash, raCtx.IntrinsicGas, logs...), nil
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

func (p *Protocol) createReceipt(
	status uint64,
	blkHeight uint64,
	actHash hash.Hash256,
	gasConsumed uint64,
	logs ...*action.Log,
) *action.Receipt {
	// TODO: need to review the fields
	return &action.Receipt{
		Status:          status,
		BlockHeight:     blkHeight,
		ActionHash:      actHash,
		GasConsumed:     gasConsumed,
		ContractAddress: p.addr.String(),
		Logs:            logs,
	}
}
