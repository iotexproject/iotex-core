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
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
)

func calculateTxRoot(acts []action.SealedEnvelope) hash.Hash256 {
	h := make([]hash.Hash256, 0, len(acts))
	for _, act := range acts {
		h = append(h, act.Hash())
	}
	if len(h) == 0 {
		return hash.ZeroHash256
	}
	return crypto.NewMerkleTree(h).HashTree()
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
	// TODO (zhi): figure out why the block height should be larger than 0
	if blk.Height() > 0 {
		// verify new block's signature is correct
		if !blk.VerifySignature() {
			return errors.Errorf(
				"failed to verify block's signature with public key: %x",
				blk.PublicKey(),
			)
		}
	}

	hashExpect := blk.TxRoot()
	hashActual := blk.CalculateTxRoot()
	if !bytes.Equal(hashExpect[:], hashActual[:]) {
		return errors.Errorf(
			"wrong tx hash %x, expecting %x",
			hashActual,
			hashExpect,
		)
	}
	return nil
}
