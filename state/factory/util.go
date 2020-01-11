// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
)

// createGenesisStates initialize the genesis states
func createGenesisStates(ctx context.Context, ws WorkingSet) error {
	if bcCtx, ok := protocol.GetBlockchainCtx(ctx); ok {
		for _, p := range bcCtx.Registry.All() {
			if gsc, ok := p.(protocol.GenesisStateCreator); ok {
				if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
					return errors.Wrap(err, "failed to create genesis states for protocol")
				}
			}
		}
	}

	return ws.Finalize()
}

func runActions(ctx context.Context, ws WorkingSet, actions []action.SealedEnvelope) ([]*action.Receipt, WorkingSet, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	bcCtx.History = ws.History()
	ctx = protocol.WithBlockchainCtx(ctx, bcCtx)
	registry := bcCtx.Registry
	for _, p := range registry.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, nil, err
			}
		}
	}
	// TODO: verify whether the post system actions are appended tail

	receipts, err := ws.RunActions(ctx, actions)
	if err != nil {
		return nil, nil, err
	}
	return receipts, ws, ws.Finalize()
}

func pickAndRunActions(
	ctx context.Context,
	ws WorkingSet,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) ([]*action.Receipt, []action.SealedEnvelope, WorkingSet, error) {
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]action.SealedEnvelope, 0)

	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	registry := bcCtx.Registry
	for _, p := range registry.All() {
		if pp, ok := p.(protocol.PreStatesCreator); ok {
			if err := pp.CreatePreStates(ctx, ws); err != nil {
				return nil, nil, nil, err
			}
		}
	}

	// initial action iterator
	actionIterator := actioniterator.NewActionIterator(actionMap)
	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}
		receipt, err := ws.RunAction(ctx, nextAction)
		if err != nil {
			if errors.Cause(err) == action.ErrHitGasLimit {
				// hit block gas limit, we should not process actions belong to this user anymore since we
				// need monotonically increasing nonce. But we can continue processing other actions
				// that belong other users
				actionIterator.PopAccount()
				continue
			}
			return nil, nil, nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextAction.Hash())
		}
		if receipt != nil {
			blkCtx.GasLimit -= receipt.GasConsumed
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, nextAction)

		// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
		// than certain threshold
		if blkCtx.GasLimit < allowedBlockGasResidue {
			break
		}
	}
	for _, selp := range postSystemActions {
		receipt, err := ws.RunAction(ctx, selp)
		if err != nil {
			return nil, nil, nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, selp)
	}

	return receipts, executedActions, ws, ws.Finalize()
}

func simulateExecution(
	ctx context.Context,
	ws WorkingSet,
	caller address.Address,
	ex *action.Execution,
	getBlockHash evm.GetBlockHash,
) ([]byte, *action.Receipt, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{
			Caller: caller,
		},
	)
	zeroAddr, err := address.FromString(address.ZeroAddress)
	if err != nil {
		return nil, nil, err
	}
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    bcCtx.Tip.Height + 1,
			BlockTimeStamp: time.Time{},
			GasLimit:       bcCtx.Genesis.BlockGasLimit,
			Producer:       zeroAddr,
		},
	)

	return evm.ExecuteContract(
		ctx,
		ws,
		ex,
		getBlockHash,
	)
}
