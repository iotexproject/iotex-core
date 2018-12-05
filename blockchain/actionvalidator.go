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
)

type actionValidator struct {
	blk      *Block
	ws       factory.WorkingSet
	bc       *blockchain
	gasLimit *uint64
}

// newActionValidator return a new action validator
func newActionValidator(blk *Block, ws factory.WorkingSet, bc *blockchain, gasLimit *uint64) *actionValidator {
	return &actionValidator{
		blk:      blk,
		ws:       ws,
		bc:       bc,
		gasLimit: gasLimit,
	}
}

// Next load next action of account of top action
func (av *actionValidator) Validate(bestAction action.Action) error {
	var err error
	switch bestAction.(type) {
	case *action.Transfer:
		gas, err := bestAction.IntrinsicGas()
		if err != nil {
			break
		}
		if *(av.gasLimit) < gas {
			err = action.ErrHitGasLimit
			break
		}
		*(av.gasLimit) -= gas
	case *action.Vote:
		gas, err := bestAction.IntrinsicGas()
		if err != nil {
			break
		}
		if *(av.gasLimit) < gas {
			err = action.ErrHitGasLimit
			break
		}
		*(av.gasLimit) -= gas
	case *action.Execution:
		var receipt *action.Receipt
		receipt, err = evm.ExecuteContract(av.blk.Height(), av.blk.HashBlock(), av.blk.Header.Pubkey, av.blk.Header.Timestamp().Unix(),
			av.ws, bestAction.(*action.Execution), av.bc, av.gasLimit, av.bc.config.Chain.EnableGasCharge)
		// will hash change after convert action to execution
		av.blk.Receipts[bestAction.Hash()] = receipt
	}

	return err
}
