// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"context"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Validator is the interface of validator
type Validator interface {
	// Validate validates the given block's content
	Validate(block *Block, tipHeight uint64, tipHash hash.Hash32B, containCoinbase bool) error
	// AddActionValidators add validators
	AddActionValidators(...protocol.ActionValidator)
}

type validator struct {
	sf               factory.Factory
	validatorAddr    string
	actionValidators []protocol.ActionValidator
}

var (
	// ErrInvalidTipHeight is the error returned when the block height is not valid
	ErrInvalidTipHeight = errors.New("invalid tip height")
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidBlock = errors.New("failed to validate the block")
	// ErrActionNonce is the error when the nonce of the action is wrong
	ErrActionNonce = errors.New("invalid action nonce")
	// ErrGasHigherThanLimit indicates the error of gas value
	ErrGasHigherThanLimit = errors.New("invalid gas for action")
	// ErrInsufficientGas indicates the error of insufficient gas value for data storage
	ErrInsufficientGas = errors.New("insufficient intrinsic gas value")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
	// ErrDKGSecretProposal indicates the error of DKG secret proposal
	ErrDKGSecretProposal = errors.New("invalid DKG secret proposal")
)

// Validate validates the given block's content
func (v *validator) Validate(blk *Block, tipHeight uint64, tipHash hash.Hash32B, containCoinbase bool) error {
	if err := verifyHeightAndHash(blk, tipHeight, tipHash); err != nil {
		return errors.Wrap(err, "failed to verify block's height and hash")
	}
	if err := verifySigAndRoot(blk); err != nil {
		return errors.Wrap(err, "failed to verify block's signature and merkle root")
	}

	if v.sf != nil {
		return v.verifyActions(blk, containCoinbase)
	}

	return nil
}

// AddActionValidators add validators
func (v *validator) AddActionValidators(validators ...protocol.ActionValidator) {
	v.actionValidators = append(v.actionValidators, validators...)
}

func (v *validator) verifyActions(blk *Block, containCoinbase bool) error {
	// Verify transfers, votes, executions, witness, and secrets
	confirmedNonceMap := make(map[string]uint64)
	accountNonceMap := make(map[string][]uint64)
	producerPKHash := keypair.HashPubKey(blk.Header.Pubkey)
	producerAddr := address.New(blk.Header.chainID, producerPKHash[:])
	var coinbaseCounter int
	var correctVerifications uint64
	var expectedVerifications uint64
	var actionError error
	var wg sync.WaitGroup

	for _, act := range blk.Actions {
		// TODO: Maybe need more strict measurement to validate a coinbase transfer
		if act.SrcAddr() == "" {
			coinbaseCounter++
		} else {
			// Store the nonce of the sender and verify later
			if _, ok := confirmedNonceMap[act.SrcAddr()]; !ok {
				accountNonce, err := v.sf.Nonce(act.SrcAddr())
				if err != nil {
					return errors.Wrap(err, "failed to get the confirmed nonce of action sender")
				}
				confirmedNonceMap[act.SrcAddr()] = accountNonce
				accountNonceMap[act.SrcAddr()] = make([]uint64, 0)
			}
		}

		ctx := protocol.WithValidateActionsCtx(context.Background(),
			&protocol.ValidateActionsCtx{
				NonceTracker: accountNonceMap,
				BlockHeight:  blk.Height(),
				ProducerAddr: producerAddr.IotxAddress(),
			})

		for _, validator := range v.actionValidators {
			wg.Add(1)
			expectedVerifications++
			go func(validator protocol.ActionValidator, act action.Action, counter *uint64) {
				defer wg.Done()
				if err := validator.Validate(ctx, act); err != nil {
					actionError = err
					return
				}
				atomic.AddUint64(counter, uint64(1))
			}(validator, act, &correctVerifications)
		}
	}

	wg.Wait()

	if correctVerifications != expectedVerifications {
		return errors.Wrap(actionError, "failed to validate action")
	}

	if containCoinbase && coinbaseCounter != 1 || !containCoinbase && coinbaseCounter != 0 {
		return errors.New("wrong number of coinbase transfers in block")
	}

	// Verify Witness
	if blk.SecretWitness != nil {
		// Verify witness sender address
		if _, err := iotxaddress.GetPubkeyHash(blk.SecretWitness.SrcAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate witness sender's address %s", blk.SecretWitness.SrcAddr())
		}
		// Store the nonce of the witness sender and verify later
		if _, ok := confirmedNonceMap[blk.SecretWitness.SrcAddr()]; !ok {
			accountNonce, err := v.sf.Nonce(blk.SecretWitness.SrcAddr())
			if err != nil {
				return errors.Wrap(err, "failed to get the nonce of secret sender")
			}
			confirmedNonceMap[blk.SecretWitness.SrcAddr()] = accountNonce
			accountNonceMap[blk.SecretWitness.SrcAddr()] = make([]uint64, 0)
		}
		accountNonceMap[blk.SecretWitness.SrcAddr()] = append(accountNonceMap[blk.SecretWitness.SrcAddr()], blk.SecretWitness.Nonce())
	}

	// Verify Secrets
	for _, sp := range blk.SecretProposals {
		// Verify address
		if _, err := iotxaddress.GetPubkeyHash(sp.SrcAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate secret sender's address %s", sp.SrcAddr())
		}
		if _, err := iotxaddress.GetPubkeyHash(sp.DstAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate secret recipient's address %s", sp.DstAddr())
		}

		// Store the nonce of the sender and verify later
		if _, ok := confirmedNonceMap[sp.SrcAddr()]; !ok {
			accountNonce, err := v.sf.Nonce(sp.SrcAddr())
			if err != nil {
				return errors.Wrap(err, "failed to get the nonce of secret sender")
			}
			confirmedNonceMap[sp.SrcAddr()] = accountNonce
			accountNonceMap[sp.SrcAddr()] = make([]uint64, 0)
		}
		accountNonceMap[sp.SrcAddr()] = append(accountNonceMap[sp.SrcAddr()], sp.Nonce())

		// verify secret if the validator is recipient
		if v.validatorAddr == sp.DstAddr() {
			validatorID := iotxaddress.CreateID(v.validatorAddr)
			result, err := crypto.DKG.ShareVerify(validatorID, sp.Secret(), blk.SecretWitness.Witness())
			if err == nil {
				err = ErrDKGSecretProposal
			}
			if !result {
				return errors.Wrap(err, "failed to verify the DKG secret share")
			}
		}
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
					return errors.Wrap(action.ErrNonce, "action nonce is not continuously increasing")
				}
			}
		}
	}
	return nil
}

func verifyHeightAndHash(blk *Block, tipHeight uint64, tipHash hash.Hash32B) error {
	if blk == nil {
		return ErrInvalidBlock
	}
	// verify new block has height incremented by 1
	if blk.Header.height != 0 && blk.Header.height != tipHeight+1 {
		return errors.Wrapf(
			ErrInvalidTipHeight,
			"wrong block height %d, expecting %d",
			blk.Header.height,
			tipHeight+1)
	}
	// verify new block has correctly linked to current tip
	if blk.Header.prevBlockHash != tipHash {
		return errors.Wrapf(
			ErrInvalidBlock,
			"wrong prev hash %x, expecting %x",
			blk.Header.prevBlockHash,
			tipHash)
	}
	return nil
}

func verifySigAndRoot(blk *Block) error {
	if blk.Header.height > 0 {
		// verify new block's signature is correct
		blkHash := blk.HashBlock()
		if !crypto.EC283.Verify(blk.Header.Pubkey, blkHash[:], blk.Header.blockSig) {
			return errors.Wrapf(
				ErrInvalidBlock,
				"failed to verify block's signature with public key: %x",
				blk.Header.Pubkey)
		}
	}

	hashExpect := blk.Header.txRoot
	hashActual := blk.CalculateTxRoot()
	if !bytes.Equal(hashExpect[:], hashActual[:]) {
		return errors.Wrapf(
			ErrInvalidBlock,
			"wrong tx hash %x, expecting %x",
			hashActual,
			hashActual)
	}
	return nil
}
