// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
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

func validateWithWorkingset(ctx context.Context, ws WorkingSet, blk *block.Block) error {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if err := validateNonce(ws, blk); err != nil {
		return errors.Wrap(err, "failed to validate nonce")
	}
	receipts, ws, err := runActions(ctx, ws, blk.RunnableActions().Actions())
	if err != nil {
		log.L().Panic("Failed to update state.", zap.Uint64("tipHeight", bcCtx.Tip.Height), zap.Error(err))
	}

	digest, err := ws.Digest()
	if err != nil {
		return err
	}
	if err = blk.VerifyDeltaStateDigest(digest); err != nil {
		return errors.Wrap(err, "failed to verify delta state digest")
	}
	if err = blk.VerifyReceiptRoot(calculateReceiptRoot(receipts)); err != nil {
		return errors.Wrap(err, "Failed to verify receipt root")
	}

	blk.Receipts = receipts
	return nil
}

func validateNonce(ws WorkingSet, blk *block.Block) error {
	accountNonceMap := make(map[string][]uint64)
	for _, selp := range blk.Actions {
		caller, err := address.FromBytes(selp.SrcPubkey().Hash())
		if err != nil {
			return err
		}
		appendActionIndex(accountNonceMap, caller.String(), selp.Nonce())
	}

	// Special handling for genesis block
	if blk.Height() == 0 {
		return nil
	}
	// Verify each account's Nonce
	for srcAddr, receivedNonces := range accountNonceMap {
		confirmedState, err := accountutil.AccountState(ws, srcAddr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the confirmed nonce of address %s", srcAddr)
		}
		receivedNonces := receivedNonces
		sort.Slice(receivedNonces, func(i, j int) bool { return receivedNonces[i] < receivedNonces[j] })
		for i, nonce := range receivedNonces {
			if nonce != confirmedState.Nonce+uint64(i+1) {
				return errors.Wrapf(
					action.ErrNonce,
					"the %d nonce %d of address %s (confirmed nonce %d) is not continuously increasing",
					i,
					nonce,
					srcAddr,
					confirmedState.Nonce,
				)
			}
		}
	}
	return nil
}

func appendActionIndex(accountNonceMap map[string][]uint64, srcAddr string, nonce uint64) {
	if nonce == 0 {
		return
	}
	if _, ok := accountNonceMap[srcAddr]; !ok {
		accountNonceMap[srcAddr] = make([]uint64, 0)
	}
	accountNonceMap[srcAddr] = append(accountNonceMap[srcAddr], nonce)
}

func runActions(ctx context.Context, ws WorkingSet, actions []action.SealedEnvelope) ([]*action.Receipt, WorkingSet, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
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

func createBuilderWithWorkingset(
	ctx context.Context,
	ws WorkingSet,
	actionMap map[string][]action.SealedEnvelope,
	postSystemActions []action.SealedEnvelope,
	allowedBlockGasResidue uint64,
) (*block.Builder, error) {
	rc, actions, ws, err := pickAndRunActions(ctx, ws, actionMap, postSystemActions, allowedBlockGasResidue)
	if err != nil {
		return nil, err
	}

	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()

	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	prevBlkHash := bcCtx.Tip.Hash
	// The first block's previous block hash is pointing to the digest of genesis config. This is to guarantee all nodes
	// could verify that they start from the same genesis
	if blkCtx.BlockHeight == 1 {
		prevBlkHash = bcCtx.Genesis.Hash()
	}
	digest, err := ws.Digest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get digest")
	}

	blkBuilder := block.NewBuilder(ra).
		SetHeight(blkCtx.BlockHeight).
		SetTimestamp(blkCtx.BlockTimeStamp).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(digest).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
		SetLogsBloom(calculateLogsBloom(ctx, rc))
	return blkBuilder, nil
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

func calculateReceiptRoot(receipts []*action.Receipt) hash.Hash256 {
	if len(receipts) == 0 {
		return hash.ZeroHash256
	}
	h := make([]hash.Hash256, 0, len(receipts))
	for _, receipt := range receipts {
		h = append(h, receipt.Hash())
	}
	res := crypto.NewMerkleTree(h).HashTree()
	return res
}

func calculateLogsBloom(ctx context.Context, receipts []*action.Receipt) bloom.BloomFilter {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	if blkCtx.BlockHeight < bcCtx.Genesis.AleutianBlockHeight {
		return nil
	}
	bloom, _ := bloom.NewBloomFilter(2048, 3)
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			for _, topic := range log.Topics {
				bloom.Add(topic[:])
			}
		}
	}
	return bloom
}

// generateWorkingSetCacheKey generates hash key for workingset cache by hashing blockheader core and producer pubkey
func generateWorkingSetCacheKey(blkHeader block.Header, producerAddr string) hash.Hash256 {
	sum := append(blkHeader.SerializeCore(), []byte(producerAddr)...)
	return hash.Hash256b(sum)
}
