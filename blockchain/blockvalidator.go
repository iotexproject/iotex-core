// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	cp "github.com/iotexproject/iotex-core/crypto"
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

	if blk.Header.height > 0 {
		// verify new block's signature is correct
		blkHash := blk.HashBlock()
		if !cp.Verify(blk.Header.Pubkey, blkHash[:], blk.Header.blockSig) {
			return errors.Wrapf(
				ErrInvalidBlock,
				"Fail to verify block's signature with public key: %x",
				blk.Header.Pubkey,
				tipHash)
		}
	}

	hashExpect := blk.Header.txRoot
	hashActual := blk.TxRoot()
	if bytes.Compare(hashExpect[:], hashActual[:]) != 0 {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong tx hash %x, expecting %x",
			hashActual,
			hashActual)
	}

	if v.sf != nil {
		// Verify the signatures here (balance is checked in CommitStateChanges)
		verifyCount := len(blk.Transfers) + len(blk.Votes) - 1
		if blk.Header.height == 0 {
			verifyCount++
		}
		var wg sync.WaitGroup
		wg.Add(verifyCount)
		var correctVerify uint64
		coinbaseCount := 0
		for _, tsf := range blk.Transfers {
			if tsf.IsCoinbase {
				if coinbaseCount > 1 {
					return errors.Wrapf(
						ErrInvalidBlock,
						"Wrong number of coinbase transfers")
				}
				address, err := iotxaddress.GetAddress(blk.Header.Pubkey, iotxaddress.IsTestnet, iotxaddress.ChainID)
				if err != nil {
					return errors.Wrap(err, "error when getting the address of the block head")
				}
				if address.RawAddress != tsf.Recipient {
					return errors.Wrap(action.ErrTransferError, "coinbase transfer recipient address is corrupted")
				}
				coinbaseCount++
				continue
			}
			go func(tsf *action.Transfer, correctTsf *uint64) {
				defer wg.Done()
				address, err := iotxaddress.GetAddress(tsf.SenderPublicKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
				if err != nil {
					return
				}
				if err := tsf.Verify(address); err != nil {
					return
				}
				atomic.AddUint64(correctTsf, uint64(1))
			}(tsf, &correctVerify)
		}
		for _, vote := range blk.Votes {
			go func(vote *action.Vote, correctVote *uint64) {
				defer wg.Done()
				selfPublicKey, err := vote.SelfPublicKey()
				if err != nil {
					return
				}
				address, err := iotxaddress.GetAddress(selfPublicKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
				if err != nil {
					return
				}
				if err := vote.Verify(address); err != nil {
					return
				}
				atomic.AddUint64(correctVote, uint64(1))
			}(vote, &correctVerify)
		}
		wg.Wait()
		if correctVerify != uint64(verifyCount) {
			return errors.Wrapf(
				ErrInvalidBlock,
				"Failed to verify actions signature")
		}
		if (blk.Header.height != 0 && coinbaseCount != 1) || (blk.Header.height == 0 && coinbaseCount != 0) {
			return errors.Wrapf(
				ErrInvalidBlock,
				"Wrong number of coinbase transfers")
		}
	}

	return nil
}
