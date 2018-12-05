// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/CoderZhi/go-ethereum/core/vm"
)

type actionPicker struct {
	blk      		*Block
	ws       		factory.WorkingSet
	bc       		*blockchain
	gasLimit 		*uint64
	actionIterator 	actioniterator.ActionIterator
}

// newActionPicker return a new action picker
func newActionPicker(blk *Block, ws factory.WorkingSet, bc *blockchain, gasLimit *uint64, actionIterator actioniterator.ActionIterator) *actionPicker {
	return &actionPicker{
		blk:      		blk,
		ws:      	 	ws,
		bc:       		bc,
		gasLimit: 		gasLimit,
		actionIterator: actionIterator,
	}
}

// Next load next action of account of top action
func (ap *actionPicker) PickAction(bestAction action.Action) ([]action.Action, map[hash.Hash32B]*action.Receipt) {
	appliedActionList := make([]action.Action, 0)
	actionReceipt := make(map[hash.Hash32B]*action.Receipt, 0)

	for {
		nextAction := ap.actionIterator.Next()
		if nextAction == nil {
			break
		}

		var err error
		switch nextAction.(type) {
		case *action.Transfer:
			gas, err := nextAction.IntrinsicGas()
			if err != nil {
				break
			}
			if *(ap.gasLimit) < gas {
				err = action.ErrHitGasLimit
				break
			}
			*(ap.gasLimit) -= gas
		case *action.Vote:
			gas, err := nextAction.IntrinsicGas()
			if err != nil {
				break
			}
			if *(ap.gasLimit) < gas {
				err = action.ErrHitGasLimit
				break
			}
			*(ap.gasLimit) -= gas
		case *action.Execution:
			var receipt *action.Receipt
			receipt, err = evm.ExecuteContract(ap.blk.Height(), ap.blk.HashBlock(), ap.blk.Header.Pubkey, ap.blk.Header.Timestamp().Unix(),
				ap.ws, nextAction.(*action.Execution), ap.bc, ap.gasLimit, ap.bc.config.Chain.EnableGasCharge)
			// will hash change after convert action to execution
			//ap.blk.Receipts[nextAction.Hash()] = receipt
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

	return appliedActionList, actionReceipt
}

