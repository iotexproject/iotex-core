// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func calculateTxRoot(acts []*action.SealedEnvelope) (hash.Hash256, error) {
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
		return hash.ZeroHash256, nil
	}
	return crypto.NewMerkleTree(h).HashTree(), nil
}

// calculateTransferAmount returns the calculated transfer amount
func calculateTransferAmount(acts []*action.SealedEnvelope) *big.Int {
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
