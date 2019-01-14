// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
)

// PickAction returns picked action list
func PickAction(gasLimit uint64, actionIterator actioniterator.ActionIterator) ([]action.SealedEnvelope, error) {
	pickedActions := make([]action.SealedEnvelope, 0)

	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}

		// use gaslimit for now, will change to real gas later
		gas := nextAction.GasLimit()
		if gasLimit < gas {
			break
		}
		gasLimit -= gas
		pickedActions = append(pickedActions, nextAction)
	}

	return pickedActions, nil
}
