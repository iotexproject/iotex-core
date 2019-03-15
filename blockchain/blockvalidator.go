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

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Validator is the interface of validator
type Validator interface {
	// Validate validates the given block's content
	Validate(block *block.Block, tipHeight uint64, tipHash hash.Hash256) error
	// AddActionValidators add validators
	AddActionValidators(...protocol.ActionValidator)
	AddActionEnvelopeValidators(...protocol.ActionEnvelopeValidator)
}

type validator struct {
	sf                       factory.Factory
	validatorAddr            string
	actionEnvelopeValidators []protocol.ActionEnvelopeValidator
	actionValidators         []protocol.ActionValidator
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
)

// Validate validates the given block's content
func (v *validator) Validate(blk *block.Block, tipHeight uint64, tipHash hash.Hash256) error {
	if err := verifyHeightAndHash(blk, tipHeight, tipHash); err != nil {
		return errors.Wrap(err, "failed to verify block's height and hash")
	}
	if err := verifySigAndRoot(blk); err != nil {
		return errors.Wrap(err, "failed to verify block's signature and merkle root")
	}

	if v.sf != nil {
		return v.validateActionsOnly(
			blk.Actions,
			blk.PublicKey(),
			blk.Height(),
		)
	}

	return nil
}

// AddActionValidators add validators
func (v *validator) AddActionValidators(validators ...protocol.ActionValidator) {
	v.actionValidators = append(v.actionValidators, validators...)
}

// AddActionEnvelopeValidators add action envelope validators
func (v *validator) AddActionEnvelopeValidators(validators ...protocol.ActionEnvelopeValidator) {
	v.actionEnvelopeValidators = append(v.actionEnvelopeValidators, validators...)
}

func (v *validator) validateActionsOnly(
	actions []action.SealedEnvelope,
	pk keypair.PublicKey,
	height uint64,
) error {
	// Verify transfers, votes, executions, witness, and secrets
	errChan := make(chan error, len(actions))
	accountNonceMap := make(map[string][]uint64)

	if err := v.validateActions(
		actions,
		pk,
		height,
		accountNonceMap,
		errChan,
	); err != nil {
		close(errChan)
		return err
	}

	close(errChan)
	for err := range errChan {
		return errors.Wrap(err, "failed to validate action")
	}

	// Special handling for genesis block
	if height == 0 {
		return nil
	}
	//Verify each account's Nonce
	for srcAddr, receivedNonces := range accountNonceMap {
		confirmedNonce, err := v.sf.Nonce(srcAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
		}
		receivedNonces := receivedNonces
		sort.Slice(receivedNonces, func(i, j int) bool { return receivedNonces[i] < receivedNonces[j] })
		for i, nonce := range receivedNonces {
			if nonce != confirmedNonce+uint64(i+1) {
				return errors.Wrapf(
					action.ErrNonce,
					"the %d nonce %d of address %s (confirmed nonce %d) is not continuously increasing",
					i,
					nonce,
					srcAddr,
					confirmedNonce,
				)
			}
		}
	}
	return nil
}

func (v *validator) validateActions(
	actions []action.SealedEnvelope,
	pk keypair.PublicKey,
	height uint64,
	accountNonceMap map[string][]uint64,
	errChan chan error,
) error {
	producerAddr, err := address.FromBytes(pk.Hash())
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, selp := range actions {
		caller, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		appendActionIndex(accountNonceMap, caller.String(), selp.Nonce())
		ctx := protocol.WithValidateActionsCtx(
			context.Background(),
			protocol.ValidateActionsCtx{
				BlockHeight:  height,
				ProducerAddr: producerAddr.String(),
				Caller:       caller,
			},
		)

		for _, validator := range v.actionEnvelopeValidators {
			wg.Add(1)
			go func(validator protocol.ActionEnvelopeValidator, selp action.SealedEnvelope) {
				defer wg.Done()
				if err := validator.Validate(ctx, selp); err != nil {
					errChan <- err
					return
				}
			}(validator, selp)
		}

		for _, validator := range v.actionValidators {
			wg.Add(1)
			go func(validator protocol.ActionValidator, act action.Action) {
				defer wg.Done()
				if err := validator.Validate(ctx, act); err != nil {
					errChan <- err
					return
				}
			}(validator, selp.Action())
		}
	}
	wg.Wait()

	return nil
}

func verifyHeightAndHash(blk *block.Block, tipHeight uint64, tipHash hash.Hash256) error {
	if blk == nil {
		return ErrInvalidBlock
	}
	// verify new block has height incremented by 1
	if blk.Height() != 0 && blk.Height() != tipHeight+1 {
		return errors.Wrapf(
			ErrInvalidTipHeight,
			"wrong block height %d, expecting %d",
			blk.Height(),
			tipHeight+1)
	}
	// verify new block has correctly linked to current tip
	if blk.PrevHash() != tipHash {
		blk.HeaderLogger(log.L()).Error("Previous block hash doesn't match.",
			log.Hex("expectedBlockHash", tipHash[:]))
		return errors.Wrapf(
			ErrInvalidBlock,
			"wrong prev hash %x, expecting %x",
			blk.PrevHash(),
			tipHash)
	}
	return nil
}

func verifySigAndRoot(blk *block.Block) error {
	if blk.Height() > 0 {
		// verify new block's signature is correct
		if !blk.VerifySignature() {
			return errors.Wrapf(
				ErrInvalidBlock,
				"failed to verify block's signature with public key: %x",
				blk.PublicKey())
		}
	}

	hashExpect := blk.TxRoot()
	hashActual := blk.CalculateTxRoot()
	if !bytes.Equal(hashExpect[:], hashActual[:]) {
		return errors.Wrapf(
			ErrInvalidBlock,
			"wrong tx hash %x, expecting %x",
			hashActual,
			hashExpect)
	}
	return nil
}

func appendActionIndex(accountNonceMap map[string][]uint64, srcAddr string, nonce uint64) {
	if nonce == 0 {
		return
	}
	if _, ok := accountNonceMap[srcAddr]; !ok {
		accountNonceMap[srcAddr] = make([]uint64, 0)
	}
	accountNonceMap[srcAddr] = append(accountNonceMap[srcAddr], nonce)
}
