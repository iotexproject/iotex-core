// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// handleTransfer handles a transfer
func (p *Protocol) handleTransfer(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil, nil
	}
	// check sender
	sender, err := accountutil.LoadOrCreateAccount(sm, actionCtx.Caller.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of sender %s", actionCtx.Caller.String())
	}

	gasFee := big.NewInt(0).Mul(tsf.GasPrice(), big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
	if big.NewInt(0).Add(tsf.Amount(), gasFee).Cmp(sender.Balance) == 1 {
		return nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"sender %s balance %s, required amount %s",
			actionCtx.Caller.String(),
			sender.Balance,
			big.NewInt(0).Add(tsf.Amount(), gasFee),
		)
	}

	var (
		depositLog *action.TransactionLog
		fCtx       = protocol.MustGetFeatureCtx(ctx)
	)
	if !fCtx.FixDoubleChargeGas {
		// charge sender gas
		if err := sender.SubBalance(gasFee); err != nil {
			return nil, errors.Wrapf(err, "failed to charge the gas for sender %s", actionCtx.Caller.String())
		}
		if p.depositGas != nil {
			depositLog, err = p.depositGas(ctx, sm, gasFee)
			if err != nil {
				return nil, err
			}
		}
	}

	var recipientAddr address.Address
	if fCtx.TolerateLegacyAddress {
		recipientAddr, err = address.FromStringLegacy(tsf.Recipient())
	} else {
		recipientAddr, err = address.FromString(tsf.Recipient())
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode recipient address %s", tsf.Recipient())
	}
	recipientAcct, err := accountutil.LoadAccount(sm, recipientAddr)
	if !fCtx.TolerateLegacyAddress {
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load address %s", tsf.Recipient())
		}
	}
	if err == nil && recipientAcct.IsContract() {
		// update sender Nonce
		accountutil.SetNonce(tsf, sender)
		// put updated sender's state to trie
		if err := accountutil.StoreAccount(sm, actionCtx.Caller, sender); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		if fCtx.FixDoubleChargeGas {
			if p.depositGas != nil {
				depositLog, err = p.depositGas(ctx, sm, gasFee)
				if err != nil {
					return nil, err
				}
			}
		}
		receipt := &action.Receipt{
			Status:          uint64(iotextypes.ReceiptStatus_Failure),
			BlockHeight:     blkCtx.BlockHeight,
			ActionHash:      actionCtx.ActionHash,
			GasConsumed:     actionCtx.IntrinsicGas,
			ContractAddress: p.addr.String(),
		}
		receipt.AddTransactionLogs(depositLog)
		return receipt, nil
	}

	// update sender Balance
	if err := sender.SubBalance(tsf.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the Balance of sender %s", actionCtx.Caller.String())
	}
	// update sender Nonce
	accountutil.SetNonce(tsf, sender)
	// put updated sender's state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller, sender); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}
	// check recipient
	recipient, err := accountutil.LoadOrCreateAccount(sm, tsf.Recipient())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of recipient %s", tsf.Recipient())
	}
	recipient.AddBalance(tsf.Amount())
	// put updated recipient's state to trie
	if err := accountutil.StoreAccount(sm, recipientAddr, recipient); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	if fCtx.FixDoubleChargeGas {
		if p.depositGas != nil {
			depositLog, err = p.depositGas(ctx, sm, gasFee)
			if err != nil {
				return nil, err
			}
		}
	}

	receipt := &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}
	receipt.AddTransactionLogs(&action.TransactionLog{
		Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
		Sender:    actionCtx.Caller.String(),
		Recipient: tsf.Recipient(),
		Amount:    tsf.Amount(),
	}, depositLog)

	return receipt, nil
}

// validateTransfer validates a transfer
func (p *Protocol) validateTransfer(ctx context.Context, act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	// Reject oversized transfer
	if tsf.TotalSize() > TransferSizeLimit {
		return action.ErrOversizedData
	}
	var (
		fCtx = protocol.MustGetFeatureCtx(ctx)
		err  error
	)
	if fCtx.TolerateLegacyAddress {
		_, err = address.FromStringLegacy(tsf.Recipient())
	} else {
		_, err = address.FromString(tsf.Recipient())
	}
	return err
}
