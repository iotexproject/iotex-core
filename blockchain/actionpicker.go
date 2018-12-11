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
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type actionPicker struct {
	ctx            context.Context
	ws             factory.WorkingSet
	bc             *blockchain
	gasLimit       *uint64
	actionIterator actioniterator.ActionIterator
}

// newActionPicker return a new action picker
func newActionPicker(ctx context.Context, ws factory.WorkingSet, bc *blockchain, gasLimit *uint64, actionIterator actioniterator.ActionIterator) *actionPicker {
	return &actionPicker{
		ctx:            ctx,
		ws:             ws,
		bc:             bc,
		gasLimit:       gasLimit,
		actionIterator: actionIterator,
	}
}

// Next load next action of account of top action
func (ap *actionPicker) PickAction() ([]action.Action, map[hash.Hash32B]*action.Receipt, error) {
	appliedActionList := make([]action.Action, 0)
	actionReceipt := make(map[hash.Hash32B]*action.Receipt, 0)
	raCtx, ok := state.GetRunActionsCtx(ap.ctx)
	if !ok {
		return nil, nil, errors.New("failed to get action context")
	}

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
			receipt, err = evm.ExecuteContract(raCtx.BlockHeight, raCtx.BlockHash, raCtx.ProducerPubKey, raCtx.BlockTimeStamp,
				ap.ws, nextAction.(*action.Execution), ap.bc, ap.gasLimit, ap.bc.config.Chain.EnableGasCharge)
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
