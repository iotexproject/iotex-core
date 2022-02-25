package cmd

import (
	"context"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// This Var is useless currently
	SyncHeight = &cobra.Command{
		Use:   "syncheight",
		Short: "Sync stateDB height to height x",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newHeight, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				panic(err)
			}
			svr := NewMiniServer(loadConfig())
			dao := svr.BlockDao()
			sf := svr.Factory()
			initCtx := svr.Context()

			if err := checkSanity(dao, sf, newHeight); err != nil {
				panic(err)
			}

			return syncToHeight(initCtx, dao, sf, newHeight)
		},
	}
)

func checkSanity(dao blockdao.BlockDAO, indexer blockdao.BlockIndexer, newHeight uint64) error {
	daoHeight, err := dao.Height()
	if err != nil {
		return err
	}
	indexerHeight, err := indexer.Height()
	if err != nil {
		return err
	}

	if newHeight > daoHeight {
		return errors.New("calibrated Height shouldn't be larger than the height of chainDB")
	}
	if indexerHeight > newHeight {
		return errors.New("the height of indexer shouldn't be larger than newheight")
	}
	return nil
}

func syncToHeight(
	ctx context.Context,
	dao blockdao.BlockDAO,
	indexer blockdao.BlockIndexer,
	newHeight uint64,
) error {
	indexerHeight, err := indexer.Height()
	if err != nil {
		panic(err)
	}
	tipBlk, err := dao.GetBlockByHeight(indexerHeight)
	if err != nil {
		return err
	}
	syncCtx := ctx
	for i := indexerHeight + 1; i <= newHeight; i++ {
		blk, err := getBlockFromChainDB(dao, i)
		if err != nil {
			return err
		}
		indexCtx := syncCtx
		if err := indexBlock(indexCtx, tipBlk, blk, indexer); err != nil {
			return err
		}
		c := color.New(color.FgRed).Add(color.Bold)
		c.Println("Sync indexer height to", i)
		tipBlk = blk
	}
	return nil
}

func getBlockFromChainDB(dao blockdao.BlockDAO, height uint64) (*block.Block, error) {
	blk, err := dao.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	if blk.Receipts == nil {
		blk.Receipts, err = dao.GetReceipts(height)
		if err != nil {
			return nil, err
		}
	}
	return blk, nil
}

func indexBlock(
	ctx context.Context,
	tipBlk *block.Block,
	blk *block.Block,
	indexer blockdao.BlockIndexer,
) error {
	if tipBlk.Height()+1 != blk.Height() {
		return errors.New("fail")
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	gCtx := genesis.MustExtractGenesisContext(ctx)
	bcCtx.Tip.Height = tipBlk.Height()
	if bcCtx.Tip.Height > 0 {
		bcCtx.Tip.Hash = tipBlk.HashHeader()
		bcCtx.Tip.Timestamp = tipBlk.Timestamp()
	} else {
		bcCtx.Tip.Hash = gCtx.Hash()
		bcCtx.Tip.Timestamp = time.Unix(gCtx.Timestamp, 0)
	}
	producer := blk.PublicKey().Address()
	if producer == nil {
		return errors.New("failed to get address")
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
