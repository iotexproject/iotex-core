// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"bytes"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
)

func calculateTxRoot(acts []action.SealedEnvelope) (hash.Hash256, error) {
	h := make([]hash.Hash256, 0, len(acts))
	for _, act := range acts {
		actHash, err := act.Hash()
		if err != nil {
			log.L().Debug("Error in getting hash", zap.Error(err))
			return hash.ZeroHash256, err
		}
		h = append(h, actHash)
	}
	if len(h) == 0 {
		return hash.ZeroHash256, errors.Errorf("hash length is zero")
	}
	return crypto.NewMerkleTree(h).HashTree(), nil
}

// calculateTransferAmount returns the calculated transfer amount
func calculateTransferAmount(acts []action.SealedEnvelope) *big.Int {
	transferAmount := big.NewInt(0)
	for _, act := range acts {
		transfer, ok := act.Action().(*action.Transfer)
		if !ok {
			continue
		}
		transferAmount.Add(transferAmount, transfer.Amount())
	}
	return transferAmount
}

// VerifyBlock verifies the block signature and tx root
func VerifyBlock(blk *Block) error {
	// verify new block's signature is correct
	if !blk.VerifySignature() {
		return errors.Errorf(
			"failed to verify block's signature with public key: %x",
			blk.PublicKey(),
		)
	}

	hashExpect := blk.TxRoot()
	hashActual, err := blk.CalculateTxRoot()
	if err != nil {
		log.L().Debug("error in getting hash", zap.Error(err))
		return err
	}
	if !bytes.Equal(hashExpect[:], hashActual[:]) {
		return errors.Errorf(
			"wrong tx hash %x, expecting %x",
			hashActual,
			hashExpect,
		)
	}
	return nil
}
