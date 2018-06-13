// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txpool

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/statefactory"
)

const (
	// TransferSizeLimit is the maximum size of transfer allowed
	TransferSizeLimit = 32 * 1024
	// VoteSizeLimit is the maximum size of vote allowed
	VoteSizeLimit = 224
	// GlobalSlots indicate maximum number of actions the whole actpool can hold
	GlobalSlots = 8192
	// AccountSlots indicate maximum number of an account queue can hold
	AccountSlots = 256
)

var (
	// ErrInvalidAddr is the error that the address format is invalid, cannot be decoded
	ErrInvalidAddr = errors.New("address format is invalid")
	// ErrActPool indicates the error of actpool
	ErrActPool = errors.New("invalid actpool")
	// ErrNonce indicates the error of nonce
	ErrNonce = errors.New("invalid nonce")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
)

// ActPool is the interface of actpool
type ActPool interface {
	// Reset resets actpool state
	Reset()
	// PickActs returns all currently accepted transfers and votes in actpool
	PickActs() ([]*action.Transfer, []*action.Vote)
	// AddTsf adds an transfer into the pool after passing validation
	AddTsf(tsf *action.Transfer) error
	// AddVote adds a vote into the pool after passing validation
	AddVote(vote *action.Vote) error
}

// actPool implements ActPool interface
type actPool struct {
	mutex       sync.RWMutex
	sf          statefactory.StateFactory
	accountActs map[string]ActQueue
	allActions  map[common.Hash32B]*iproto.ActionPb
}

// NewActPool constructs a new actpool
func NewActPool(sf statefactory.StateFactory) ActPool {
	ap := &actPool{
		sf:          sf,
		accountActs: make(map[string]ActQueue),
		allActions:  make(map[common.Hash32B]*iproto.ActionPb),
	}
	return ap
}

// Reset resets actpool state
// Step I: remove all the actions in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update pending nonce and confirmed nonce in each account if it still exists in pool
// Specifically, first synchronize old confirmed nonce with committed nonce in order to prevent omitting reevaluation of
// uncommitted but confirmed actions in pool after update of pending balance
// Then starting from the current committed nonce, iteratively update pending nonce if nonces are consecutive as well as
// confirmed nonce if pending balance is sufficient
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	// Remove committed actions in actpool
	ap.removeCommittedActs()
	for from, queue := range ap.accountActs {
		// Reset pending balance for each account
		balance, err := ap.sf.Balance(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		queue.SetPendingBalance(balance)

		// Reset confirmed nonce and pending nonce for each account
		committedNonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting Tsf")
			return
		}
		confirmedNonce := committedNonce + 1
		queue.SetConfirmedNonce(confirmedNonce)
		queue.UpdateNonce(confirmedNonce)
	}
}

// PickActs returns all currently accepted transfers and votes for all accounts
func (ap *actPool) PickActs() ([]*action.Transfer, []*action.Vote) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	transfers := []*action.Transfer{}
	votes := []*action.Vote{}
	for _, queue := range ap.accountActs {
		for _, act := range queue.ConfirmedActs() {
			switch {
			case act.GetTransfer() != nil:
				tsf := action.Transfer{}
				tsf.ConvertFromTransferPb(act.GetTransfer())
				transfers = append(transfers, &tsf)
			case act.GetVote() != nil:
				vote := action.Vote{}
				vote.ConvertFromVotePb(act.GetVote())
				votes = append(votes, &vote)
			}
		}
	}
	return transfers, votes
}

// AddTsf inserts a new transfer into account queue if it passes validation
func (ap *actPool) AddTsf(tsf *action.Transfer) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	hash := tsf.Hash()
	// Reject transfer if it already exists in pool
	if ap.allActions[hash] != nil {
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting existed transfer")
		return fmt.Errorf("existed transfer: %x", hash)
	}
	// Reject transfer if it fails validation
	if err := ap.validateTsf(tsf); err != nil {
		logger.Error().
			Bytes("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid transfer")
		return err
	}
	// Reject transfer if pool space is full
	if uint64(len(ap.allActions)) >= GlobalSlots {
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting transfer due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transfer")
	}
	// Wrap tsf as an action
	action := &iproto.ActionPb{&iproto.ActionPb_Transfer{tsf.ConvertToTransferPb()}}
	return ap.addAction(tsf.Sender, action, hash, tsf.Nonce)
}

// AddVote inserts a new vote into account queue if it passes validation
func (ap *actPool) AddVote(vote *action.Vote) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	hash := vote.Hash()
	// Reject vote if it already exists in pool
	if ap.allActions[hash] != nil {
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting existed vote")
		return fmt.Errorf("existed vote: %x", hash)
	}
	// Reject vote if it fails validation
	if err := ap.validateVote(vote); err != nil {
		logger.Error().
			Bytes("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid vote")
		return err
	}
	// Reject vote if pool space is full
	if uint64(len(ap.allActions)) >= GlobalSlots {
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting vote due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for vote")
	}

	voter, _ := iotxaddress.GetAddress(vote.SelfPubkey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	// Wrap vote as an action
	action := &iproto.ActionPb{&iproto.ActionPb_Vote{vote.ConvertToVotePb()}}
	return ap.addAction(voter.RawAddress, action, hash, vote.Nonce)
}

//======================================
// private functions
//======================================
// validateTsf checks whether a tranfer is valid
func (ap *actPool) validateTsf(tsf *action.Transfer) error {
	// Reject oversized transfer
	if tsf.TotalSize() > TransferSizeLimit {
		logger.Error().Msg("Error when validating transfer")
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject transfer of negative amount
	if tsf.Amount.Sign() < 0 {
		logger.Error().Msg("Error when validating transfer")
		return errors.Wrapf(ErrBalance, "negative value")
	}
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(tsf.Sender)
	if pkhash == nil {
		return ErrInvalidAddr
	}

	sender, err := iotxaddress.GetAddress(tsf.SenderPublicKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating transfer")
		return errors.Wrapf(err, "invalid address")
	}
	// Verify transfer using sender's public key
	if err := tsf.Verify(sender); err != nil {
		logger.Error().Err(err).Msg("Error when validatng transfer")
		return errors.Wrapf(err, "failed to verify Transfer signature")
	}
	// Reject transfer if nonce is too low
	committedNonce, err := ap.sf.Nonce(tsf.Sender)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating Tsf")
		return errors.Wrapf(err, "invalid nonce value")
	}
	confirmedNonce := committedNonce + 1
	if confirmedNonce > tsf.Nonce {
		logger.Error().Msg("Error when validating transfer")
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// validateVote checks whether a vote is valid
func (ap *actPool) validateVote(vote *action.Vote) error {
	// Reject oversized vote
	if vote.TotalSize() > VoteSizeLimit {
		logger.Error().Msg("Error when validating vote")
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	voter, err := iotxaddress.GetAddress(vote.SelfPubkey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "invalid address")
	}
	// Verify vote using voter's public key
	if err := vote.Verify(voter); err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "failed to verify Vote signature")
	}

	// Reject vote if nonce is too low
	committedNonce, err := ap.sf.Nonce(voter.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "invalid nonce value")
	}
	confirmedNonce := committedNonce + 1
	if confirmedNonce > vote.Nonce {
		logger.Error().Msg("Error when validating vote")
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

func (ap *actPool) addAction(sender string, action *iproto.ActionPb, hash common.Hash32B, actNonce uint64) error {
	queue := ap.accountActs[sender]
	if queue == nil {
		queue = NewActQueue()
		ap.accountActs[sender] = queue
		// Initialize pending nonce and confirmed nonce for new account
		committedNonce, err := ap.sf.Nonce(sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding action")
			return err
		}
		confirmedNonce := committedNonce + 1
		queue.SetPendingNonce(confirmedNonce)
		queue.SetConfirmedNonce(confirmedNonce)
		// Initialize balance for new account
		balance, err := ap.sf.Balance(sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding action")
			return err
		}
		queue.SetPendingBalance(balance)
	}
	if queue.Overlaps(action) {
		// Nonce already exists
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting action because replacement action is not supported")
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}

	if queue.Len() >= AccountSlots {
		logger.Error().
			Bytes("hash", hash[:]).
			Msg("Rejecting action due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for action")
	}
	queue.Put(action)
	ap.allActions[hash] = action
	// If the pending nonce equals this nonce, update pending nonce
	nonce := queue.PendingNonce()
	if actNonce == nonce {
		queue.UpdateNonce(actNonce)
	}
	return nil
}

// removeCommittedActs removes processed (committed to block) actions from pool
func (ap *actPool) removeCommittedActs() {
	for from, queue := range ap.accountActs {
		committedNonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing committed actions")
			return
		}
		confirmedNonce := committedNonce + 1
		// Remove all actions that are committed to new block
		for _, act := range queue.FilterNonce(confirmedNonce) {
			var hash common.Hash32B
			switch {
			case act.GetTransfer() != nil:
				tsf := &action.Transfer{}
				tsf.ConvertFromTransferPb(act.GetTransfer())
				hash = tsf.Hash()
			case act.GetVote() != nil:
				vote := &action.Vote{}
				vote.ConvertFromVotePb(act.GetVote())
				hash = vote.Hash()
			}
			logger.Info().
				Bytes("hash", hash[:]).
				Msg("Removed committed action")
			delete(ap.allActions, hash)
		}
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountActs, from)
		}
	}
}
