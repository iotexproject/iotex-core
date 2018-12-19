// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/pkg/errors"

	"github.com/CoderZhi/go-ethereum/core/vm"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
)

// PickAction returns picked action list
func PickAction(ctx context.Context, ws factory.WorkingSet, bc *blockchain, gasLimit *uint64, actionIterator actioniterator.ActionIterator) ([]action.SealedEnvelope, map[hash.Hash32B]*action.Receipt, error) {
	appliedActionList := make([]action.SealedEnvelope, 0)
	actionReceipt := make(map[hash.Hash32B]*action.Receipt)
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return nil, nil, errors.New("failed to get action context")
	}

	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}

		var err error
		switch nextAction.Action().(type) {
		case *action.Transfer:
			gas, err := nextAction.IntrinsicGas()
			if err != nil {
				break
			}
			if *(gasLimit) < gas {
				err = action.ErrHitGasLimit
				break
			}
			*(gasLimit) -= gas
		case *action.Vote:
			gas, err := nextAction.IntrinsicGas()
			if err != nil {
				break
			}
			if *(gasLimit) < gas {
				err = action.ErrHitGasLimit
				break
			}
			*(gasLimit) -= gas
		case *action.Execution:
			var receipt *action.Receipt
			receipt, err = evm.ExecuteContract(raCtx.BlockHeight, raCtx.BlockHash, raCtx.ProducerPubKey, raCtx.BlockTimeStamp,
				ws, nextAction.Action().(*action.Execution), bc, gasLimit, bc.config.Chain.EnableGasCharge)
			// will hash change after convert action to execution
			actionReceipt[nextAction.Hash()] = receipt
		}
		if err == nil {
			appliedActionList = append(appliedActionList, nextAction)
		}

		// error handling
		if err == action.ErrHitGasLimit || err == action.ErrOutOfGas || err == vm.ErrOutOfGas {
			break
		} else {
			// do not handle other erros
			appliedActionList = append(appliedActionList, nextAction)
		}
	}

	return appliedActionList, actionReceipt, nil
}
