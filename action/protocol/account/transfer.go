// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// handleTransfer handles a transfer
func (p *Protocol) handleTransfer(act action.Action, sm protocol.StateManager) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	if tsf.IsContract() {
		return nil
	}
	if !tsf.IsCoinbase() {
		// check sender
		sender, err := LoadOrCreateAccount(sm, tsf.Sender(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of sender %s", tsf.Sender())
		}
		if tsf.Amount().Cmp(sender.Balance) == 1 {
			return errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance of sender %s", tsf.Sender())
		}
		// update sender Balance
		if err := sender.SubBalance(tsf.Amount()); err != nil {
			return errors.Wrapf(err, "failed to update the Balance of sender %s", tsf.Sender())
		}
		// update sender Nonce
		SetNonce(tsf, sender)
		// put updated sender's state to trie
		if err := StoreAccount(sm, tsf.Sender(), sender); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update sender votes
		if len(sender.Votee) > 0 {
			// sender already voted to a different person
			voteeOfSender, err := LoadOrCreateAccount(sm, sender.Votee, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of sender's votee %s", sender.Votee)
			}
			voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tsf.Amount())
			// put updated state of sender's votee to trie
			if err := StoreAccount(sm, sender.Votee, voteeOfSender); err != nil {
				return errors.Wrap(err, "failed to update pending account changes to trie")
			}
		}
	}
	// check recipient
	recipient, err := LoadOrCreateAccount(sm, tsf.Recipient(), big.NewInt(0))
	if err != nil {
		return errors.Wrapf(err, "failed to load or create the account of recipient %s", tsf.Recipient())
	}
	if err := recipient.AddBalance(tsf.Amount()); err != nil {
		return errors.Wrapf(err, "failed to update the Balance of recipient %s", tsf.Recipient())
	}
	// put updated recipient's state to trie
	if err := StoreAccount(sm, tsf.Recipient(), recipient); err != nil {
		return errors.Wrap(err, "failed to update pending account changes to trie")
	}
	// Update recipient votes
	if len(recipient.Votee) > 0 {
		// recipient already voted to a different person
		voteeOfRecipient, err := LoadOrCreateAccount(sm, recipient.Votee, big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of recipient's votee %s", recipient.Votee)
		}
		voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tsf.Amount())
		// put updated state of recipient's votee to trie
		if err := StoreAccount(sm, recipient.Votee, voteeOfRecipient); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
	}
	return nil
}

// validateTransfer validates a transfer
func (p *Protocol) validateTransfer(act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	// Reject coinbase transfer
	if tsf.IsCoinbase() {
		return errors.Wrap(action.ErrTransfer, "coinbase transfer")
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
	if _, err := iotxaddress.GetPubkeyHash(tsf.Recipient()); err != nil {
		return errors.Wrapf(err, "error when validating recipient's address %s", tsf.Recipient())
	}
	return nil
}
