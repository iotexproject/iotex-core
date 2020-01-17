// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Minter is the interface of block minters
type Minter interface {
	// Mint mints the block by given actionMap and timestamp
	Mint(context.Context, map[string][]action.SealedEnvelope) (*block.Block, error)
}

type minter struct {
	sf               factory.Factory
	minterPrivateKey crypto.PrivateKey
}

func (m *minter) Mint(ctx context.Context, actionMap map[string][]action.SealedEnvelope) (*block.Block, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	// run execution and update state trie root hash
	postSystemActions := make([]action.SealedEnvelope, 0)
	for _, p := range bcCtx.Registry.All() {
		if psac, ok := p.(protocol.PostSystemActionsCreator); ok {
			elps, err := psac.CreatePostSystemActions(ctx)
			if err != nil {
				return nil, err
			}
			for _, elp := range elps {
				se, err := action.Sign(elp, m.minterPrivateKey)
				if err != nil {
					return nil, err
				}
				postSystemActions = append(postSystemActions, se)
			}
		}
	}
	rc, actions, ws, err := m.sf.PickAndRunActions(ctx, actionMap, postSystemActions)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to update state changes in new block %d", blkCtx.BlockHeight)
	}
	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()

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
	blk, err := block.NewBuilder(ra).
		SetHeight(blkCtx.BlockHeight).
		SetTimestamp(blkCtx.BlockTimeStamp).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(digest).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
		SetLogsBloom(calculateLogsBloom(ctx, rc)).
		SignAndBuild(m.minterPrivateKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}
	blk.WorkingSet = ws
	return &blk, nil
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
