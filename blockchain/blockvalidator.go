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
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Validator is the interface of validator
type Validator interface {
	// Validate validates the given block's content
	Validate(block *block.Block, tipHeight uint64, tipHash hash.Hash32B, containCoinbase bool) error
	// ValidateActionsOnly validates the actions only
	ValidateActionsOnly(
		actions []action.SealedEnvelope,
		containCoinbase bool,
		secretWitness *action.SecretWitness,
		secretProposals []*action.SecretProposal,
		pk keypair.PublicKey,
		chainID uint32,
		height uint64,
	) error
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
	// ErrDKGSecretProposal indicates the error of DKG secret proposal
	ErrDKGSecretProposal = errors.New("invalid DKG secret proposal")
)

// Validate validates the given block's content
func (v *validator) Validate(blk *block.Block, tipHeight uint64, tipHash hash.Hash32B, containCoinbase bool) error {
	if err := verifyHeightAndHash(blk, tipHeight, tipHash); err != nil {
		return errors.Wrap(err, "failed to verify block's height and hash")
	}
	if err := verifySigAndRoot(blk); err != nil {
		return errors.Wrap(err, "failed to verify block's signature and merkle root")
	}

	if v.sf != nil {
		return v.ValidateActionsOnly(
			blk.Actions,
			containCoinbase,
			blk.SecretWitness,
			blk.SecretProposals,
			blk.PublicKey(),
			blk.ChainID(),
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

func (v *validator) ValidateActionsOnly(
	actions []action.SealedEnvelope,
	containCoinbase bool,
	secretWitness *action.SecretWitness,
	secretProposals []*action.SecretProposal,
	pk keypair.PublicKey,
	chainID uint32,
	height uint64,
) error {
	// Verify transfers, votes, executions, witness, and secrets
	errChan := make(chan error, len(actions))
	accountNonceMap := make(map[string][]uint64)

	if err := v.validateActions(
		actions,
		containCoinbase,
		secretWitness,
		secretProposals,
		pk,
		chainID,
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

	if height > 0 {
		//Verify each account's Nonce
		for srcAddr, receivedNonces := range accountNonceMap {
			confirmedNonce, err := v.sf.Nonce(srcAddr)
			if err != nil {
				return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
			}
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
	}
	return nil
}

func (v *validator) validateActions(
	actions []action.SealedEnvelope,
	containCoinbase bool,
	secretWitness *action.SecretWitness,
	secretProposals []*action.SecretProposal,
	pk keypair.PublicKey,
	chainID uint32,
	height uint64,
	accountNonceMap map[string][]uint64,
	errChan chan error,
) error {
	coinbaseCounter := 0
	producerPK := keypair.HashPubKey(pk)
	producerAddr := address.New(chainID, producerPK[:])

	var wg sync.WaitGroup
	for _, selp := range actions {
		// TODO: Maybe need more strict measurement to validate a coinbase transfer
		if act, ok := selp.Action().(*action.Transfer); ok && act.IsCoinbase() {
			coinbaseCounter++
		} else {
			appendActionIndex(accountNonceMap, selp.SrcAddr(), selp.Nonce())
		}

		ctx := protocol.WithValidateActionsCtx(context.Background(),
			&protocol.ValidateActionsCtx{
				BlockHeight:  height,
				ProducerAddr: producerAddr.IotxAddress(),
			})

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

	if containCoinbase && coinbaseCounter != 1 || !containCoinbase && coinbaseCounter != 0 {
		return errors.New("wrong number of coinbase transfers in block")
	}

	// Verify Witness
	if secretWitness != nil {
		// Verify witness sender address
		if _, err := iotxaddress.GetPubkeyHash(secretWitness.SrcAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate witness sender's address %s", secretWitness.SrcAddr())
		}
		appendActionIndex(accountNonceMap, secretWitness.SrcAddr(), secretWitness.Nonce())
	}

	// Verify Secrets
	for _, sp := range secretProposals {
		// Verify address
		if _, err := iotxaddress.GetPubkeyHash(sp.SrcAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate secret sender's address %s", sp.SrcAddr())
		}
		if _, err := iotxaddress.GetPubkeyHash(sp.DstAddr()); err != nil {
			return errors.Wrapf(err, "failed to validate secret recipient's address %s", sp.DstAddr())
		}
		appendActionIndex(accountNonceMap, sp.SrcAddr(), sp.Nonce())

		// verify secret if the validator is recipient
		if v.validatorAddr == sp.DstAddr() {
			validatorID := iotxaddress.CreateID(v.validatorAddr)
			result, err := crypto.DKG.ShareVerify(validatorID, sp.Secret(), secretWitness.Witness())
			if err == nil {
				err = ErrDKGSecretProposal
			}
			if !result {
				return errors.Wrap(err, "failed to verify the DKG secret share")
			}
		}
	}
	return nil
}

func verifyHeightAndHash(blk *block.Block, tipHeight uint64, tipHash hash.Hash32B) error {
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
	if _, ok := accountNonceMap[srcAddr]; !ok {
		accountNonceMap[srcAddr] = make([]uint64, 0)
	}
	accountNonceMap[srcAddr] = append(accountNonceMap[srcAddr], nonce)
}
