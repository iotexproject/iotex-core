// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// handleTransfer handles a transfer
func (p *Protocol) handleTransfer(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil, nil
	}
	if tsf.IsContract() {
		return nil, nil
	}
	// check sender
	sender, err := accountutil.LoadOrCreateAccount(sm, raCtx.Caller.String(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of sender %s", raCtx.Caller.String())
	}

	if raCtx.GasLimit < raCtx.IntrinsicGas {
		return nil, action.ErrHitGasLimit
	}

	gasFee := big.NewInt(0).Mul(tsf.GasPrice(), big.NewInt(0).SetUint64(raCtx.IntrinsicGas))
	if big.NewInt(0).Add(tsf.Amount(), gasFee).Cmp(sender.Balance) == 1 {
		return nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"sender %s balance %s, required amount %s",
			raCtx.Caller.String(),
			sender.Balance,
			big.NewInt(0).Add(tsf.Amount(), gasFee),
		)
	}

	// charge sender gas
	if err := sender.SubBalance(gasFee); err != nil {
		return nil, errors.Wrapf(err, "failed to charge the gas for sender %s", raCtx.Caller.String())
	}
	if err := rewarding.DepositGas(ctx, sm, gasFee, raCtx.Registry); err != nil {
		return nil, err
	}

	if tsf.Amount().Cmp(sender.Balance) == 1 {
		return nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"failed to verify the Balance of sender %s",
			raCtx.Caller.String(),
		)
	}
	// update sender Balance
	if err := sender.SubBalance(tsf.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the Balance of sender %s", raCtx.Caller.String())
	}
	// update sender Nonce
	accountutil.SetNonce(tsf, sender)
	// put updated sender's state to trie
	if err := accountutil.StoreAccount(sm, raCtx.Caller.String(), sender); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}
	// Update sender votes
	if len(sender.Votee) > 0 {
		// sender already voted to a different person
		voteeOfSender, err := accountutil.LoadOrCreateAccount(sm, sender.Votee, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of sender's votee %s", sender.Votee)
		}
		voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tsf.Amount())
		// put updated state of sender's votee to trie
		if err := accountutil.StoreAccount(sm, sender.Votee, voteeOfSender); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}

		// Update candidate
		if voteeOfSender.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, raCtx.BlockHeight, sender.Votee, voteeOfSender.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}
	// check recipient
	recipient, err := accountutil.LoadOrCreateAccount(sm, tsf.Recipient(), big.NewInt(0))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of recipient %s", tsf.Recipient())
	}
	if err := recipient.AddBalance(tsf.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the Balance of recipient %s", tsf.Recipient())
	}

	// put updated recipient's state to trie
	if err := accountutil.StoreAccount(sm, tsf.Recipient(), recipient); err != nil {
		return nil, errors.Wrap(err, "failed to update pending account changes to trie")
	}
	// Update recipient votes
	if len(recipient.Votee) > 0 {
		// recipient already voted to a different person
		voteeOfRecipient, err := accountutil.LoadOrCreateAccount(sm, recipient.Votee, big.NewInt(0))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load or create the account of recipient's votee %s", recipient.Votee)
		}
		voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tsf.Amount())
		// put updated state of recipient's votee to trie
		if err := accountutil.StoreAccount(sm, recipient.Votee, voteeOfRecipient); err != nil {
			return nil, errors.Wrap(err, "failed to update pending account changes to trie")
		}

		if voteeOfRecipient.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, raCtx.BlockHeight, recipient.Votee, voteeOfRecipient.VotingWeight); err != nil {
				return nil, errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}
	return &action.Receipt{
		Status:      action.SuccessReceiptStatus,
		ActHash:     raCtx.ActionHash,
		GasConsumed: raCtx.IntrinsicGas,
	}, nil
}

// validateTransfer validates a transfer
func (p *Protocol) validateTransfer(_ context.Context, act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	// Reject oversized transfer
	if tsf.TotalSize() > TransferSizeLimit {
		return errors.Wrap(action.ErrActPool, "oversized data")
	}
	// Reject transfer of negative amount
	if tsf.Amount().Sign() < 0 {
		return errors.Wrap(action.ErrBalance, "negative value")
	}
	// check if recipient's address is valid
	if _, err := address.FromString(tsf.Recipient()); err != nil {
		return errors.Wrapf(err, "error when validating recipient's address %s", tsf.Recipient())
	}
	return nil
}
