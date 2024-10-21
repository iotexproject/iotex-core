// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// handleTransfer handles a transfer
func (p *Protocol) handleTransfer(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	var (
		fCtx      = protocol.MustGetFeatureCtx(ctx)
		actionCtx = protocol.MustGetActionCtx(ctx)
		blkCtx    = protocol.MustGetBlockCtx(ctx)
		tsf       = elp.Action().(*action.Transfer)
	)
	accountCreationOpts := []state.AccountCreationOption{}
	if fCtx.CreateLegacyNonceAccount {
		accountCreationOpts = append(accountCreationOpts, state.LegacyNonceAccountTypeOption())
	}
	// check sender
	sender, err := accountutil.LoadOrCreateAccount(sm, actionCtx.Caller, accountCreationOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of sender %s", actionCtx.Caller.String())
	}

	priorityFee, baseFee, err := protocol.SplitGas(ctx, elp, actionCtx.IntrinsicGas)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to split gas")
	}
	total := big.NewInt(0).Add(tsf.Amount(), priorityFee)
	total.Add(total, baseFee)
	if !sender.HasSufficientBalance(total) {
		return nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"sender %s balance %s, required amount %s",
			actionCtx.Caller.String(),
			sender.Balance,
			total,
		)
	}

	var depositLog []*action.TransactionLog
	if !fCtx.FixDoubleChargeGas {
		// charge sender gas
		if err := sender.SubBalance(baseFee); err != nil {
			return nil, errors.Wrapf(err, "failed to charge the gas for sender %s", actionCtx.Caller.String())
		}
		if p.depositGas != nil {
			depositLog, err = p.depositGas(ctx, sm, baseFee)
			if err != nil {
				return nil, err
			}
		}
	}

	if fCtx.FixGasAndNonceUpdate || elp.Nonce() != 0 {
		// update sender Nonce
		if err := sender.SetPendingNonce(elp.Nonce() + 1); err != nil {
			return nil, errors.Wrapf(err, "failed to update pending nonce of sender %s", actionCtx.Caller.String())
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
	recipientAcct, err := accountutil.LoadAccount(sm, recipientAddr, accountCreationOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load address %s", tsf.Recipient())
	}
	price := protocol.EffectiveGasPrice(ctx, elp)
	if recipientAcct.IsContract() {
		// put updated sender's state to trie
		if err := accountutil.StoreAccount(sm, actionCtx.Caller, sender); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}
		if fCtx.FixDoubleChargeGas {
			if p.depositGas != nil {
				depositLog, err = p.depositGas(ctx, sm, baseFee, protocol.PriorityFeeOption(priorityFee))
				if err != nil {
					return nil, err
				}
			}
		}
		receipt := &action.Receipt{
			Status:            uint64(iotextypes.ReceiptStatus_Failure),
			BlockHeight:       blkCtx.BlockHeight,
			ActionHash:        actionCtx.ActionHash,
			GasConsumed:       actionCtx.IntrinsicGas,
			ContractAddress:   p.addr.String(),
			EffectiveGasPrice: price,
		}
		receipt.AddTransactionLogs(depositLog...)
		return receipt, nil
	}

	// update sender Balance
	if err := sender.SubBalance(tsf.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the Balance of sender %s", actionCtx.Caller.String())
	}

	// put updated sender's state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller, sender); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	// check recipient
	recipient, err := accountutil.LoadOrCreateAccount(sm, recipientAddr, accountCreationOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of recipient %s", tsf.Recipient())
	}
	if err := recipient.AddBalance(tsf.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to add balance %s", tsf.Amount())
	}
	// put updated recipient's state to trie
	if err := accountutil.StoreAccount(sm, recipientAddr, recipient); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}

	if fCtx.FixDoubleChargeGas {
		if p.depositGas != nil {
			depositLog, err = p.depositGas(ctx, sm, baseFee, protocol.PriorityFeeOption(priorityFee))
			if err != nil {
				return nil, err
			}
		}
	}

	receipt := &action.Receipt{
		Status:            uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:       blkCtx.BlockHeight,
		ActionHash:        actionCtx.ActionHash,
		GasConsumed:       actionCtx.IntrinsicGas,
		ContractAddress:   p.addr.String(),
		EffectiveGasPrice: price,
	}
	receipt.AddTransactionLogs(&action.TransactionLog{
		Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
		Sender:    actionCtx.Caller.String(),
		Recipient: tsf.Recipient(),
		Amount:    tsf.Amount(),
	})
	receipt.AddTransactionLogs(depositLog...)

	return receipt, nil
}

// validateTransfer validates a transfer
func (p *Protocol) validateTransfer(ctx context.Context, elp action.Envelope) error {
	// Reject oversized transfer
	if elp.Size() > TransferSizeLimit {
		return action.ErrOversizedData
	}
	// already asserted the payload is action.Transfer on the call stack
	tsf := elp.Action().(*action.Transfer)
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
