// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cmd

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// syncHeight represents the syncheight command
var syncHeight = &cobra.Command{
	Use:   "sync",
	Short: "Sync stateDB height to height x",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stopHeight, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}
		svr, err := newMiniServer(miniServerConfig())
		if err != nil {
			return err
		}
		return syncToHeight(svr, stopHeight)
	},
}

func syncToHeight(svr *miniServer, stopHeight uint64) error {
	var (
		ctx     = svr.Context()
		dao     = svr.BlockDao()
		indexer = svr.Factory()
	)

	checker := blockdao.NewBlockIndexerChecker(dao)
	if err := checker.CheckIndexer(ctx, indexer, stopHeight-1, func(height uint64) {
		if height%5000 == 0 {
			log.L().Info(
				"indexer is catching up.",
				zap.Uint64("height", height-1),
			)
		}
	}); err != nil {
		return err
	}
	log.L().Info("indexer is up to date.", zap.Uint64("height", stopHeight-1))

	stopHeightIndexer, err := svr.StopHeightFactory(stopHeight)
	if err != nil {
		return err
	}
	if err := putBlock(ctx, dao, stopHeightIndexer, stopHeight); err != nil {
		return err
	}
	log.L().Info("indexer is up to date.", zap.Uint64("height", stopHeight))
	return nil
}

func putBlock(
	ctx context.Context,
	dao blockdao.BlockDAO,
	indexer blockdao.BlockIndexer,
	height uint64,
) error {
	blk, err := dao.GetBlockByHeight(height)
	if err != nil {
		return err
	}
	if blk.Receipts == nil {
		blk.Receipts, err = dao.GetReceipts(height)
		if err != nil {
			return err
		}
	}
	producer := blk.PublicKey().Address()
	if producer == nil {
		return errors.New("failed to get address")
	}

	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	gCtx := genesis.MustExtractGenesisContext(ctx)
	bcCtx.Tip.Height = blk.Height()
	if bcCtx.Tip.Height > 0 {
		bcCtx.Tip.Hash = blk.HashHeader()
		bcCtx.Tip.Timestamp = blk.Timestamp()
	} else {
		bcCtx.Tip.Hash = gCtx.Hash()
		bcCtx.Tip.Timestamp = time.Unix(gCtx.Timestamp, 0)
	}

	for {
		var err error
		if err = indexer.PutBlock(protocol.WithBlockCtx(
			protocol.WithBlockchainCtx(ctx, bcCtx),
			protocol.BlockCtx{
				BlockHeight:    blk.Height(),
				BlockTimeStamp: blk.Timestamp(),
				Producer:       producer,
				GasLimit:       gCtx.BlockGasLimit,
			},
		), blk); err == nil {
			break
		}
		if errors.Cause(err) == block.ErrDeltaStateMismatch {
			log.L().Info("delta state mismatch", zap.Uint64("block", blk.Height()))
			continue
		}
		return err
	}
	return nil
}
