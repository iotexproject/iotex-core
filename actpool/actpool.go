// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// TransferSizeLimit is the maximum size of transfer allowed
	TransferSizeLimit = 32 * 1024
	// VoteSizeLimit is the maximum size of vote allowed
	VoteSizeLimit = 262
)

var (
	// ErrActPool indicates the error of actpool
	ErrActPool = errors.New("invalid actpool")
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("invalid transfer")
	// ErrNonce indicates the error of nonce
	ErrNonce = errors.New("invalid nonce")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
	// ErrVotee indicates the error of votee
	ErrVotee = errors.New("votee is not a candidate")
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
	// GetPendingNonce returns pending nonce in pool given an account address
	GetPendingNonce(addr string) (uint64, error)
	// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
	GetUnconfirmedActs(addr string) []*iproto.ActionPb
}

// actPool implements ActPool interface
type actPool struct {
	mutex sync.RWMutex
	// maxNumActPerPool indicates maximum number of actions the whole actpool can hold
	maxNumActPerPool uint64
	// maxNumActPerAcct indicates maximum number of actions an account queue can hold
	maxNumActPerAcct uint64
	bc               blockchain.Blockchain
	accountActs      map[string]ActQueue
	allActions       map[hash.Hash32B]*iproto.ActionPb
}

// NewActPool constructs a new actpool
func NewActPool(bc blockchain.Blockchain, cfg config.ActPool) (ActPool, error) {
	if bc == nil {
		return nil, errors.New("Try to attach a nil blockchain")
	}
	ap := &actPool{
		maxNumActPerPool: cfg.MaxNumActPerPool,
		maxNumActPerAcct: cfg.MaxNumActPerAcct,
		bc:               bc,
		accountActs:      make(map[string]ActQueue),
		allActions:       make(map[hash.Hash32B]*iproto.ActionPb),
	}
	return ap, nil
}

// Reset resets actpool state
// Step I: remove all the actions in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update queue's status in each account and remove invalid actions following queue's update
// Specifically, first reset the pending nonce based on confirmed nonce in order to prevent omitting reevaluation of
// unconfirmed but pending actions in pool after update of pending balance
// Then starting from the current confirmed nonce, iteratively update pending nonce if nonces are consecutive and pending
// balance is sufficient, and remove all the subsequent actions once the pending balance becomes insufficient
func (ap *actPool) Reset() {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	// Remove confirmed actions in actpool
	ap.removeConfirmedActs()
	for from, queue := range ap.accountActs {
		// Reset pending balance for each account
		balance, err := ap.bc.Balance(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		queue.SetPendingBalance(balance)

		// Reset pending nonce and remove invalid actions for each account
		confirmedNonce, err := ap.bc.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when resetting actpool state")
			return
		}
		pendingNonce := confirmedNonce + 1
		queue.SetStartNonce(pendingNonce)
		queue.SetPendingNonce(pendingNonce)
		ap.updateAccount(from)
	}
}

// PickActs returns all currently accepted transfers and votes for all accounts
func (ap *actPool) PickActs() ([]*action.Transfer, []*action.Vote) {
	ap.mutex.Lock()
	defer ap.mutex.Unlock()

	transfers := make([]*action.Transfer, 0)
	votes := make([]*action.Vote, 0)
	for _, queue := range ap.accountActs {
		for _, act := range queue.PendingActs() {
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
			Hex("hash", hash[:]).
			Msg("Rejecting existed transfer")
		return fmt.Errorf("existed transfer: %x", hash)
	}
	// Reject transfer if it fails validation
	if err := ap.validateTsf(tsf); err != nil {
		logger.Error().
			Hex("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid transfer")
		return err
	}
	// Reject transfer if pool space is full
	if uint64(len(ap.allActions)) >= ap.maxNumActPerPool {
		logger.Warn().
			Hex("hash", hash[:]).
			Msg("Rejecting transfer due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for transfer")
	}
	// Wrap tsf as an action
	action := &iproto.ActionPb{Action: &iproto.ActionPb_Transfer{tsf.ConvertToTransferPb()}}
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
			Hex("hash", hash[:]).
			Msg("Rejecting existed vote")
		return fmt.Errorf("existed vote: %x", hash)
	}
	// Reject vote if it fails validation
	if err := ap.validateVote(vote); err != nil {
		logger.Error().
			Hex("hash", hash[:]).
			Err(err).
			Msg("Rejecting invalid vote")
		return err
	}
	// Reject vote if pool space is full
	if uint64(len(ap.allActions)) >= ap.maxNumActPerPool {
		logger.Warn().
			Hex("hash", hash[:]).
			Msg("Rejecting vote due to insufficient space")
		return errors.Wrapf(ErrActPool, "insufficient space for vote")
	}

	selfPublicKey, _ := vote.SelfPublicKey()
	voter, _ := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPublicKey)
	// Wrap vote as an action
	action := &iproto.ActionPb{Action: &iproto.ActionPb_Vote{vote.ConvertToVotePb()}}
	return ap.addAction(voter.RawAddress, action, hash, vote.Nonce)
}

// GetPendingNonce returns pending nonce in pool or confirmed nonce given an account address
func (ap *actPool) GetPendingNonce(addr string) (uint64, error) {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.PendingNonce(), nil
	}
	confirmedNonce, err := ap.bc.Nonce(addr)
	pendingNonce := confirmedNonce + 1
	return pendingNonce, err
}

// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
func (ap *actPool) GetUnconfirmedActs(addr string) []*iproto.ActionPb {
	if queue, ok := ap.accountActs[addr]; ok {
		return queue.AllActs()
	}
	return make([]*iproto.ActionPb, 0)
}

//======================================
// private functions
//======================================
// validateTsf checks whether a tranfer is valid
func (ap *actPool) validateTsf(tsf *action.Transfer) error {
	// Reject coinbase transfer
	if tsf.IsCoinbase {
		logger.Error().Msg("Error when validating transfer")
		return errors.Wrapf(ErrTransfer, "coinbase transfer")
	}
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
	_, err := iotxaddress.GetPubkeyHash(tsf.Sender)
	if err != nil {
		return errors.Wrap(err, "error when getting the pubkey hash")
	}

	sender, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, tsf.SenderPublicKey)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating transfer")
		return errors.Wrapf(err, "invalid address")
	}
	// Verify transfer using sender's public key
	if err := tsf.Verify(sender); err != nil {
		logger.Error().Err(err).Msg("Error when validating transfer")
		return errors.Wrapf(err, "failed to verify Transfer signature")
	}
	// Reject transfer if nonce is too low
	confirmedNonce, err := ap.bc.Nonce(tsf.Sender)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating transfer")
		return errors.Wrapf(err, "invalid nonce value")
	}
	pendingNonce := confirmedNonce + 1
	if pendingNonce > tsf.Nonce {
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
	selfPublicKey, err := vote.SelfPublicKey()
	if err != nil {
		return err
	}
	voter, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPublicKey)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "invalid voter address")
	}
	// Verify vote using voter's public key
	if err := vote.Verify(voter); err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "failed to verify Vote signature")
	}

	// Reject vote if nonce is too low
	confirmedNonce, err := ap.bc.Nonce(voter.RawAddress)
	if err != nil {
		logger.Error().Err(err).Msg("Error when validating vote")
		return errors.Wrapf(err, "invalid nonce value")
	}
	if vote.VoteeAddress != "" {
		// Reject vote if votee is not a candidate
		if err != nil {
			logger.Error().
				Err(err).Hex("voter", vote.SelfPubkey[:]).Str("votee", vote.VoteeAddress).
				Msg("Error when validating vote")
			return errors.Wrapf(err, "invalid votee public key: %s", vote.VoteeAddress)
		}
		voteeState, err := ap.bc.StateByAddr(vote.VoteeAddress)
		if err != nil {
			logger.Error().Err(err).
				Hex("voter", vote.SelfPubkey[:]).Str("votee", vote.VoteeAddress).
				Msg("Error when validating vote")
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.VoteeAddress)
		}
		if voter.RawAddress != vote.VoteeAddress && !voteeState.IsCandidate {
			logger.Error().Err(ErrVotee).
				Hex("voter", vote.SelfPubkey[:]).Str("votee", vote.VoteeAddress).
				Msg("Error when validating vote")
			return errors.Wrapf(ErrVotee, "votee has not self-nominated: %s", vote.VoteeAddress)
		}
	}

	pendingNonce := confirmedNonce + 1
	if pendingNonce > vote.Nonce {
		logger.Error().Msg("Error when validating vote")
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

func (ap *actPool) addAction(sender string, act *iproto.ActionPb, hash hash.Hash32B, actNonce uint64) error {
	queue := ap.accountActs[sender]
	if queue == nil {
		queue = NewActQueue()
		ap.accountActs[sender] = queue
		confirmedNonce, err := ap.bc.Nonce(sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding action")
			return err
		}
		// Initialize pending nonce for new account
		pendingNonce := confirmedNonce + 1
		queue.SetPendingNonce(pendingNonce)
		queue.SetStartNonce(pendingNonce)
		// Initialize balance for new account
		balance, err := ap.bc.Balance(sender)
		if err != nil {
			logger.Error().Err(err).Msg("Error when adding action")
			return err
		}
		queue.SetPendingBalance(balance)
	}
	if queue.Overlaps(act) {
		// Nonce already exists
		logger.Error().
			Hex("hash", hash[:]).
			Msg("Rejecting action because replacement action is not supported")
		return errors.Wrapf(ErrNonce, "duplicate nonce")
	}

	if actNonce-queue.StartNonce() >= ap.maxNumActPerAcct {
		// Nonce exceeds current range
		logger.Warn().
			Hex("hash", hash[:]).
			Uint64("startNonce", queue.StartNonce()).Uint64("actNonce", actNonce).
			Msg("Rejecting action because nonce is too large")
		return errors.Wrapf(ErrNonce, "nonce too large")
	}

	if transfer := act.GetTransfer(); transfer != nil {
		tsf := &action.Transfer{}
		tsf.ConvertFromTransferPb(transfer)
		if queue.PendingBalance().Cmp(tsf.Amount) < 0 {
			// Pending balance is insufficient
			logger.Warn().
				Hex("hash", hash[:]).
				Msg("Rejecting transfer due to insufficient balance")
			return errors.Wrapf(ErrBalance, "insufficient balance for transfer")
		}
	}

	err := queue.Put(act)
	if err != nil {
		logger.Warn().
			Hex("hash", hash[:]).
			Err(err).
			Msg("cannot put act into ActQueue")
		return errors.Wrap(err, "cannot put act into ActQueue")
	}
	ap.allActions[hash] = act
	// If the pending nonce equals this nonce, update queue
	nonce := queue.PendingNonce()
	if actNonce == nonce {
		ap.updateAccount(sender)
	}
	return nil
}

// removeConfirmedActs removes processed (committed to block) actions from pool
func (ap *actPool) removeConfirmedActs() {
	for from, queue := range ap.accountActs {
		confirmedNonce, err := ap.bc.Nonce(from)
		if err != nil {
			logger.Error().Err(err).Msg("Error when removing confirmed actions")
			return
		}
		pendingNonce := confirmedNonce + 1
		// Remove all actions that are committed to new block
		acts := queue.FilterNonce(pendingNonce)
		ap.removeInvalidActs(acts)

		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			delete(ap.accountActs, from)
		}
	}
}

func (ap *actPool) removeInvalidActs(acts []*iproto.ActionPb) {
	for _, act := range acts {
		var hash hash.Hash32B
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
		logger.Debug().
			Hex("hash", hash[:]).
			Msg("Removed invalidated action")
		delete(ap.allActions, hash)
	}
}

// updateAccount updates queue's status and remove invalidated actions from pool if necessary
func (ap *actPool) updateAccount(sender string) {
	queue := ap.accountActs[sender]
	acts := queue.UpdateQueue(queue.PendingNonce())
	if len(acts) > 0 {
		ap.removeInvalidActs(acts)
	}

	// Delete the queue entry if it becomes empty
	if queue.Empty() {
		delete(ap.accountActs, sender)
	}
}
