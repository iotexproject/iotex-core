// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

// Validator is the interface of validator
type Validator interface {
	// Validate validates the given block's content
	Validate(block *Block, tipHeight uint64, tipHash hash.Hash32B) error
}

type validator struct {
	sf state.Factory
}

var (
	// ErrInvalidTipHeight is the error returned when the block height is not valid
	ErrInvalidTipHeight = errors.New("invalid tip height")
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidBlock = errors.New("failed to validate the block")
	// ErrActionNonce is the error when the nonce of the action is wrong
	ErrActionNonce = errors.New("invalid action nonce")
	// ErrGasHigherThanLimit indicates the error of gas value
	ErrGasHigherThanLimit = errors.New("invalid gas for execution")
	// ErrInsufficientGas indicates the error of insufficient gas value for data storage
	ErrInsufficientGas = errors.New("insufficient intrinsic gas value")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
)

// Validate validates the given block's content
func (v *validator) Validate(blk *Block, tipHeight uint64, tipHash hash.Hash32B) error {
	if blk == nil {
		return ErrInvalidBlock
	}
	// verify new block has height incremented by 1
	if blk.Header.height != 0 && blk.Header.height != tipHeight+1 {
		return errors.Wrapf(
			ErrInvalidTipHeight,
			"Wrong block height %d, expecting %d",
			blk.Header.height,
			tipHeight+1)
	}
	// verify new block has correctly linked to current tip
	if blk.Header.prevBlockHash != tipHash {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong prev hash %x, expecting %x",
			blk.Header.prevBlockHash,
			tipHash)
	}

	if blk.IsDummyBlock() {
		return nil
	}

	if blk.Header.height > 0 {
		// verify new block's signature is correct
		blkHash := blk.HashBlock()
		if !crypto.EC283.Verify(blk.Header.Pubkey, blkHash[:], blk.Header.blockSig) {
			return errors.Wrapf(
				ErrInvalidBlock,
				"Fail to verify block's signature with public key: %x",
				blk.Header.Pubkey,
				tipHash)
		}
	}

	hashExpect := blk.Header.txRoot
	hashActual := blk.TxRoot()
	if !bytes.Equal(hashExpect[:], hashActual[:]) {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong tx hash %x, expecting %x",
			hashActual,
			hashActual)
	}

	if v.sf != nil {
		return v.verifyActions(blk)
	}

	return nil
}

func (v *validator) verifyActions(blk *Block) error {
	// Verify transfers and votes (balance is checked in CommitStateChanges)
	confirmedNonceMap := make(map[string]uint64)
	accountNonceMap := make(map[string][]uint64)
	var wg sync.WaitGroup
	wg.Add(len(blk.Transfers) + len(blk.Votes) + len(blk.Executions))
	var correctAction uint64
	var coinbaseCount uint64
	for _, tsf := range blk.Transfers {
		if blk.Header.height > 0 && !tsf.IsCoinbase {
			// Verify Nonce
			// Verify Signature
			// Verify Coinbase transfer

			// Store the nonce of the sender and verify later
			if _, ok := confirmedNonceMap[tsf.Sender]; !ok {
				accountNonce, err := v.sf.Nonce(tsf.Sender)
				if err != nil {
					return errors.Wrap(err, "Failed to get the nonce of transfer sender")
				}
				confirmedNonceMap[tsf.Sender] = accountNonce
				accountNonceMap[tsf.Sender] = make([]uint64, 0)
			}
			accountNonceMap[tsf.Sender] = append(accountNonceMap[tsf.Sender], tsf.Nonce)
		}

		go func(tsf *action.Transfer, correctTsf *uint64, correctCoinbase *uint64) {
			defer wg.Done()
			// Verify coinbase transfer
			if tsf.IsCoinbase {
				address, err := iotxaddress.GetAddressByPubkey(
					iotxaddress.IsTestnet,
					iotxaddress.ChainID,
					blk.Header.Pubkey,
				)
				if err != nil {
					return
				}
				if address.RawAddress != tsf.Recipient {
					return
				}
				atomic.AddUint64(correctCoinbase, uint64(1))
				return
			}

			// Verify signature
			address, err := iotxaddress.GetAddressByPubkey(
				iotxaddress.IsTestnet,
				iotxaddress.ChainID,
				tsf.SenderPublicKey,
			)
			if err != nil {
				return
			}
			if err := tsf.Verify(address); err != nil {
				return
			}
			atomic.AddUint64(correctTsf, uint64(1))
		}(tsf, &correctAction, &coinbaseCount)
	}
	for _, vote := range blk.Votes {
		// Verify Nonce
		// Verify Signature
		if blk.Header.height > 0 {
			// Store the nonce of the voter and verify later
			voterAddress := vote.GetVote().VoterAddress
			if _, ok := confirmedNonceMap[voterAddress]; !ok {
				accountNonce, err := v.sf.Nonce(voterAddress)
				if err != nil {
					return errors.Wrap(err, "Failed to get the nonce of the voter")
				}
				confirmedNonceMap[voterAddress] = accountNonce
				accountNonceMap[voterAddress] = make([]uint64, 0)
			}
			accountNonceMap[voterAddress] = append(accountNonceMap[voterAddress], vote.Nonce)
		}

		// Verify signature
		go func(vote *action.Vote, correctVote *uint64) {
			defer wg.Done()
			selfPublicKey, err := vote.SelfPublicKey()
			if err != nil {
				return
			}
			address, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPublicKey)
			if err != nil {
				return
			}
			if err := vote.Verify(address); err != nil {
				return
			}
			atomic.AddUint64(correctVote, uint64(1))
		}(vote, &correctAction)
	}
	for _, execution := range blk.Executions {
		// Verify Nonce
		// Verify Signature
		// Verify Gas
		// Verify Amount
		if blk.Header.height > 0 {
			// Store the nonce of the executor and verify later
			executor := execution.Executor
			if _, ok := confirmedNonceMap[executor]; !ok {
				accountNonce, err := v.sf.Nonce(executor)
				if err != nil {
					return errors.Wrap(err, "Failed to get the nonce of the executor")
				}
				confirmedNonceMap[executor] = accountNonce
				accountNonceMap[executor] = make([]uint64, 0)
			}
			accountNonceMap[executor] = append(accountNonceMap[executor], execution.Nonce)
		}

		// Verify signature
		go func(execution *action.Execution, correctVote *uint64) {
			defer wg.Done()
			executorPubKey := execution.ExecutorPubKey
			address, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, executorPubKey)
			if err != nil {
				return
			}
			if err := execution.Verify(address); err != nil {
				return
			}
			atomic.AddUint64(correctVote, uint64(1))
		}(execution, &correctAction)

		// Reject oversized transfer
		if execution.GasLimit > GasLimit {
			return errors.Wrapf(ErrGasHigherThanLimit, "Gas is higher than gas limit")
		}
		intrinsicGas, err := IntrinsicGas(execution.Data)
		if intrinsicGas > execution.GasLimit || err != nil {
			return errors.Wrapf(ErrInsufficientGas, "insufficient gas for execution")
		}

		// Reject execution of negative amount
		if execution.Amount.Sign() < 0 {
			return errors.Wrapf(ErrBalance, "negative value")
		}
	}
	wg.Wait()
	// Verify coinbase transfer count
	if (blk.Header.height != 0 && coinbaseCount != 1) || (blk.Header.height == 0 && coinbaseCount != 0) {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong number of coinbase transfers")
	}
	if correctAction+coinbaseCount != uint64(len(blk.Transfers)+len(blk.Votes)+len(blk.Executions)) {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Failed to verify actions signature")
	}
	if blk.Header.height > 0 {
		//Verify each account's Nonce
		for address := range confirmedNonceMap {
			// The nonce of each action should be increasing, unique and consecutive
			confirmedNonce := confirmedNonceMap[address]
			receivedNonce := accountNonceMap[address]
			sort.Slice(receivedNonce, func(i, j int) bool { return receivedNonce[i] < receivedNonce[j] })
			for i, nonce := range receivedNonce {
				if nonce != confirmedNonce+uint64(i+1) {
					return errors.Wrap(ErrActionNonce, "the nonce of the action is invalid")
				}
			}
		}
	}
	return nil
}
