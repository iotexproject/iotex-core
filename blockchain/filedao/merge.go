package filedao

import (
	"context"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	// mergeDao wraps a file dao, and merge the blocks from the original dao on start
	mergeDao struct {
		FileDAO
		originalDao FileDAO
		mergeMax    uint64
	}
)

// NewMergeDao creates a new merge dao
func NewMergeDao(original FileDAO, new FileDAO, mergeMax uint64) FileDAO {
	return &mergeDao{
		FileDAO:     new,
		originalDao: original,
		mergeMax:    mergeMax,
	}
}

func (md *mergeDao) Start(ctx context.Context) error {
	if err := md.FileDAO.Start(ctx); err != nil {
		return err
	}
	if err := md.originalDao.Start(context.Background()); err != nil {
		log.L().Error("failed to start original dao", zap.Error(err))
		return nil
	}
	defer md.originalDao.Stop(ctx)

	height, err := md.FileDAO.Height()
	if err != nil {
		return err
	}
	orgTip, err := md.originalDao.Height()
	if err != nil {
		return err
	}
	// doing nothing if the height is larger than the original dao
	if height >= orgTip {
		return nil
	}
	// init the blocks from the original dao
	start := uint64(1)
	if md.mergeMax > 0 && orgTip > (md.mergeMax+1) {
		start = orgTip - md.mergeMax
		if d, ok := md.FileDAO.(interface{ SetStart(uint64) error }); ok {
			if err = d.SetStart(start); err != nil {
				return err
			}
		}
	}
	for i := start; i <= orgTip; i++ {
		blk, err := md.originalDao.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		receipts, err := md.originalDao.GetReceipts(i)
		if err != nil {
			return err
		}
		blk.Receipts = receipts
		if md.originalDao.ContainsTransactionLog() {
			logs, err := md.originalDao.TransactionLogs(i)
			if err != nil {
				return err
			}
			if err = fillTransactionLog(receipts, logs.Logs); err != nil {
				return err
			}
		}
		if err := md.PutBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}
