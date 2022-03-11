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

type syncheightCmd struct {
	svr *miniServer
}

var (
	// SyncHeight is cobra command "syncheight"
	SyncHeight = &cobra.Command{
		Use:   "syncheight",
		Short: "Sync stateDB height to height x",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newHeight, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			svr, err := newMiniServer(miniServerConfig())
			if err != nil {
				return err
			}
			syncheightCmd := newSyncheightCmd(svr)

			if err := syncheightCmd.checkSanity(newHeight); err != nil {
				return err
			}
			return syncheightCmd.syncToHeight(newHeight)
		},
	}
)

func newSyncheightCmd(svr *miniServer) *syncheightCmd {
	return &syncheightCmd{
		svr: svr,
	}
}

func (cmd *syncheightCmd) checkSanity(newHeight uint64) error {
	daoHeight, err := cmd.svr.BlockDao().Height()
	if err != nil {
		return err
	}
	indexerHeight, err := cmd.svr.Factory().Height()
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

func (cmd *syncheightCmd) syncToHeight(newHeight uint64) error {
	var (
		ctx     = cmd.svr.Context()
		dao     = cmd.svr.BlockDao()
		indexer = cmd.svr.Factory()
	)
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
