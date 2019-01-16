// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// handleTransfer handles a transfer
func (p *Protocol) handleTransfer(act action.Action, raCtx protocol.RunActionsCtx, sm protocol.StateManager) error {
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

		if raCtx.EnableGasCharge {
			// Load or create account for producer
			producer, err := LoadOrCreateAccount(sm, raCtx.ProducerAddr, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of block producer %s", raCtx.ProducerAddr)
			}
			gas, err := tsf.IntrinsicGas()
			if err != nil {
				return errors.Wrapf(err, "failed to get intrinsic gas for transfer hash %s", tsf.Hash())
			}
			if *raCtx.GasLimit < gas {
				return vm.ErrOutOfGas
			}

			gasFee := big.NewInt(0).Mul(tsf.GasPrice(), big.NewInt(0).SetUint64(gas))
			if big.NewInt(0).Add(tsf.Amount(), gasFee).Cmp(sender.Balance) == 1 {
				return errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance of sender %s", tsf.Sender())
			}

			// charge sender gas
			if err := sender.SubBalance(gasFee); err != nil {
				return errors.Wrapf(err, "failed to charge the gas for sender %s", tsf.Sender())
			}
			// compensate block producer gas
			if err := producer.AddBalance(gasFee); err != nil {
				return errors.Wrapf(err, "failed to compensate gas to producer")
			}
			// Put updated producer's state to trie
			if err := StoreAccount(sm, raCtx.ProducerAddr, producer); err != nil {
				return errors.Wrap(err, "failed to update pending account changes to trie")
			}
			*raCtx.GasLimit -= gas
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

			// Update candidate
			if voteeOfSender.IsCandidate {
				if err := candidatesutil.LoadAndUpdateCandidates(sm, sender.Votee, voteeOfSender.VotingWeight); err != nil {
					return errors.Wrap(err, "failed to load and update candidates")
				}
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

		if voteeOfRecipient.IsCandidate {
			if err := candidatesutil.LoadAndUpdateCandidates(sm, recipient.Votee, voteeOfRecipient.VotingWeight); err != nil {
				return errors.Wrap(err, "failed to load and update candidates")
			}
		}
	}
	return nil
}

// validateTransfer validates a transfer
func (p *Protocol) validateTransfer(ctx context.Context, act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	vaCtx, validateInBlock := protocol.GetValidateActionsCtx(ctx)

	if tsf.IsCoinbase() {
		if !validateInBlock {
			return errors.Wrap(action.ErrTransfer, "unexpected coinbase transfer")
		}
		if vaCtx.ProducerAddr != tsf.Recipient() {
			return errors.Wrap(action.ErrTransfer, "wrong coinbase recipient")
		}
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
	if _, err := address.Bech32ToAddress(tsf.Recipient()); err != nil {
		return errors.Wrapf(err, "error when validating recipient's address %s", tsf.Recipient())
	}
	return nil
}
