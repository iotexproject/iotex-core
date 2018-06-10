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
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/statefactory"
)

const (
	// GlobalSlots indicate maximum transfers the whole actpool can hold
	GlobalSlots = 5120
	// AccountSlots indicate maximum transfers an account can hold
	AccountSlots = 80
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
	// PickTsfs returns all currently accepted transfers in actpool
	PickTsfs() []*action.Transfer
	// AddTsf adds a transfer into the pool after validation
	AddTsf(tsf *action.Transfer) error
	// AddVote adds a vote into pool after validation
	AddVote(vote *action.Vote) error
}

// actPool implements ActPool interface
type actPool struct {
	mutex       sync.RWMutex
	sf          statefactory.StateFactory
	p2p         *network.Overlay
	accountTsfs map[string]TsfQueue
	allTsfs     map[common.Hash32B]*action.Transfer
}

// NewActPool constructs a new actpool
func NewActPool(sf statefactory.StateFactory, p2p *network.Overlay) ActPool {
	ap := &actPool{
		sf:          sf,
		p2p:         p2p,
		accountTsfs: make(map[string]TsfQueue),
		allTsfs:     make(map[common.Hash32B]*action.Transfer),
	}
	return ap
}

// Reset resets actpool state
// Step I: remove all the transfers in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update pending nonce and confirmed nonce in each account if it still exists in pool
// Specifically, first synchronize old confirmed nonce with committed nonce in order to prevent omitting reevaluation of
// uncommitted but confirmed Tsfs in pool after update of pending balance
// Then starting from the current committed nonce, iteratively update pending nonce if nonces are consecutive as well as
// confirmed nonce if pending balance is sufficient
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	// Remove committed transfers in actpool
	ap.removeCommittedTsfs()
	for from, queue := range ap.accountTsfs {
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

// PickTsfs returns all currently accepted transfers for all accounts
func (ap *actPool) PickTsfs() []*action.Transfer {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	pending := []*action.Transfer{}
	for _, queue := range ap.accountTsfs {
		pending = append(pending, queue.ConfirmedTsfs()...)
	}
	return pending
}

// validateTsf checks whether a tranfer is valid
func (ap *actPool) validateTsf(tsf *action.Transfer) error {
	// Reject oversized transfer
	if tsf.TotalSize() > 32*1024 {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject transfer of negative amount
	if tsf.Amount.Sign() < 0 {
		return errors.Wrapf(ErrBalance, "negative value")
	}
	// check sender
	pkhash := iotxaddress.GetPubkeyHash(tsf.Sender)
	if pkhash == nil {
		return ErrInvalidAddr
	}
	// Reject transfer if nonce is too low
	committedNonce, err := ap.sf.Nonce(tsf.Sender)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating Tsf")
		return err
	}
	confirmedNonce := committedNonce + 1
	if confirmedNonce > tsf.Nonce {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// AddTsf inserts a new transfer into account queue if it passes validation
func (ap *actPool) AddTsf(tsf *action.Transfer) error {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()
	hash := tsf.Hash()
	// Reject transfer if it already exists in pool
	if ap.allTsfs[hash] != nil {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting existed transfer")
		return fmt.Errorf("existed transfer: %x", hash)
	}
	// Reject transfer if it fails validation
	if err := ap.validateTsf(tsf); err != nil {
		logger.Info().
			Bytes("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid transfer")
		return err
	}
	// Reject transfer if pool space is full
	if uint64(len(ap.allTsfs)) >= GlobalSlots {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transfer due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transfer")
	}
	// check sender
	pkhash := iotxaddress.GetPubkeyHash(tsf.Sender)
	if pkhash == nil {
		return ErrInvalidAddr
	}
	queue := ap.accountTsfs[tsf.Sender]
	if queue == nil {
		queue = NewTsfQueue()
		ap.accountTsfs[tsf.Sender] = queue
		// Initialize pending nonce and confirmed nonce for new account
		committedNonce, err := ap.sf.Nonce(tsf.Sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding Tsf")
			return err
		}
		confirmedNonce := committedNonce + 1
		queue.SetPendingNonce(confirmedNonce)
		queue.SetConfirmedNonce(confirmedNonce)
		// Initialize balance for new account
		balance, err := ap.sf.Balance(tsf.Sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding Tsf")
			return err
		}
		queue.SetPendingBalance(balance)
	}

	if queue.Overlaps(tsf) {
		// Nonce already exists
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transfer because replacement Tsf is not supported")
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}

	if queue.Len() >= AccountSlots {
		logger.Info().
			Bytes("hash", hash[:]).
			Msg("Rejecting transfer due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transfer")
	}
	queue.Put(tsf)
	ap.allTsfs[hash] = tsf
	// If the pending nonce equals this nonce, update pending nonce
	nonce := queue.PendingNonce()
	if tsf.Nonce == nonce {
		queue.UpdateNonce(tsf.Nonce)
	}
	return nil
}

// AddVote inserts a new vote if it passes validation
func (ap *actPool) AddVote(vote *action.Vote) error {
	// TODO: Implement AddVote
	return nil
}

// removeCommittedTsfs removes processed (committed to block) transfers from pool
func (ap *actPool) removeCommittedTsfs() {
	for from, queue := range ap.accountTsfs {
		committedNonce, err := ap.sf.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing commited Tsfs")
			return
		}
		confirmedNonce := committedNonce + 1
		// Remove all transfers that are committed to new block
		for _, tsf := range queue.FilterNonce(confirmedNonce) {
			hash := tsf.Hash()
			logger.Info().
				Bytes("hash", hash[:]).
				Msg("Removed committed transfer")
			delete(ap.allTsfs, hash)
		}
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountTsfs, from)
		}
	}
}
