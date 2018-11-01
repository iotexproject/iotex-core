// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package transfer

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// Protocol defines the protocol of handling transfers
type Protocol struct{}

// NewProtocol instantiates the protocol of transfer
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles a transfer
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	if tsf.IsContract() {
		return nil
	}
	if !tsf.IsCoinbase() {
		// check sender
		sender, err := LoadOrCreateAccountState(ws, tsf.Sender(), big.NewInt(0))
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
		if tsf.Nonce() > sender.Nonce {
			sender.Nonce = tsf.Nonce()
		}
		// put updated sender's state to trie
		if err := StoreState(ws, tsf.Sender(), sender); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		// Update sender votes
		if len(sender.Votee) > 0 && sender.Votee != tsf.Sender() {
			// sender already voted to a different person
			voteeOfSender, err := LoadOrCreateAccountState(ws, sender.Votee, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of sender's votee %s", sender.Votee)
			}
			voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tsf.Amount())
			// put updated state of sender's votee to trie
			if err := StoreState(ws, sender.Votee, voteeOfSender); err != nil {
				return errors.Wrap(err, "failed to update pending account changes to trie")
			}
		}
	}
	// check recipient
	recipient, err := LoadOrCreateAccountState(ws, tsf.Recipient(), big.NewInt(0))
	if err != nil {
		return errors.Wrapf(err, "failed to load or create the account of recipient %s", tsf.Recipient())
	}
	if err := recipient.AddBalance(tsf.Amount()); err != nil {
		return errors.Wrapf(err, "failed to update the Balance of recipient %s", tsf.Recipient())
	}
	// put updated recipient's state to trie
	if err := StoreState(ws, tsf.Recipient(), recipient); err != nil {
		return errors.Wrap(err, "failed to update pending account changes to trie")
	}
	// Update recipient votes
	if len(recipient.Votee) > 0 && recipient.Votee != tsf.Recipient() {
		// recipient already voted to a different person
		voteeOfRecipient, err := LoadOrCreateAccountState(ws, recipient.Votee, big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of recipient's votee %s", recipient.Votee)
		}
		voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tsf.Amount())
		// put updated state of recipient's votee to trie
		if err := StoreState(ws, recipient.Votee, voteeOfRecipient); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
	}
	return nil
}

// Validate validates a transfer
func (p *Protocol) Validate(act action.Action) error {
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

// LoadOrCreateAccountState either loads an account state or creates an account state
func LoadOrCreateAccountState(ws state.WorkingSet, addr string, init *big.Int) (*state.Account, error) {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert address to public key hash")
	}
	account, err := LoadAccountState(ws, addrHash)
	switch {
	case errors.Cause(err) == state.ErrStateNotExist:
		account := state.Account{
			Balance:      init,
			VotingWeight: big.NewInt(0),
		}
		if err := ws.PutState(addrHash, &account); err != nil {
			return nil, errors.Wrapf(err, "failed to put state for account %x", addrHash)
		}
		return &account, nil
	case err != nil:
		return nil, errors.Wrapf(err, "failed to get account of %x from account trie", addrHash)
	}
	return account, nil
}

// LoadAccountState loads an account state
func LoadAccountState(ws state.WorkingSet, addrHash hash.PKHash) (*state.Account, error) {
	s, err := ws.State(addrHash, &state.Account{})
	if err == nil {
		account, ok := s.(*state.Account)
		if !ok {
			return nil, fmt.Errorf("error when casting %T state into account state", s)
		}
		return account, nil
	}
	return nil, err
}

// StoreState put updated state to trie
func StoreState(ws state.WorkingSet, addr string, state state.State) error {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to convert address to public key hash")
	}
	return ws.PutState(addrHash, state)
}
