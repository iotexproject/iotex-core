package block

import (
	"context"
	"encoding/hex"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex/indexers/models"
	"github.com/iotexproject/iotex-core/blockindex/indexers/util"
	"github.com/iotexproject/iotex-core/db"
)

const VERSION = "2.0.2"

type (
	// blockIndexer implements the Indexer interface
	blockIndexer struct {
		genesisHash hash.Hash256
		kvStore     *db.PostgresDB
	}
)

// NewBlockIndexer creates a new indexer
func NewBlockIndexer(kv db.KVStore, genesisHash hash.Hash256) (blockdao.BlockIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	pg, ok := kv.(*db.PostgresDB)
	if !ok {
		return nil, errors.New("indexer can only be created from PostgresDB")
	}
	x := blockIndexer{
		kvStore:     pg,
		genesisHash: genesisHash,
	}
	return &x, nil
}

func (b *blockIndexer) Name() string {
	return "block"
}

func (b *blockIndexer) Version() string {
	return VERSION
}

func (b *blockIndexer) Start(ctx context.Context) error {
	if err := b.kvStore.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start block plugin's db")
	}
	if err := util.Init(b.kvStore); err != nil {
		return errors.Wrap(err, "failed to init analyser db")
	}
	if err := util.AutoMigrate(b.kvStore, b.Name(), &models.Block{}); err != nil {
		return errors.Wrap(err, "failed to start block plugin")
	}

	return nil
}

func (b *blockIndexer) Stop(ctx context.Context) error {
	return nil
}

// PutBlock index the block
func (b *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	if err := b.putBlock(ctx, blk); err != nil {
		return err
	}
	return nil
}

func (b *blockIndexer) putBlock(ctx context.Context, blk *block.Block) error {
	err := b.kvStore.DB().Transaction(func(tx *gorm.DB) error {
		blkHash := blk.HashBlock()

		year, err := strconv.Atoi(blk.Timestamp().Format("2006"))
		if err != nil {
			return err
		}
		month, err := strconv.Atoi(blk.Timestamp().Format("01"))
		if err != nil {
			return err
		}
		day, err := strconv.Atoi(blk.Timestamp().Format("02"))
		if err != nil {
			return err
		}
		m := &models.Block{
			BlockHeight:     blk.Height(),
			BlockHash:       hex.EncodeToString(blkHash[:]),
			ProducerAddress: blk.ProducerAddress(),
			NumActions:      len(blk.Actions),
			Timestamp:       time.Unix(blk.Timestamp().Unix(), 0),
			Year:            year,
			Month:           month,
			Day:             day,
		}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return util.UpdateIndexHeightByTx(tx, b.Name(), blk.Height())
	})

	return err
}

// Height return the blockchain height
func (b *blockIndexer) Height() (uint64, error) {
	//depHeight, err := r.dao.Height()
	//if err != nil {
	//	return depHeight, err
	//}
	//if dep, ok := r.plugin.(plugin.DependentAdapter); ok {
	//	for _, pluginName := range dep.DependentPlugins() {
	//		pluginHeight, err := db.GetIndexHeight(pluginName)
	//		if err != nil {
	//			return pluginHeight, err
	//		}
	//		if depHeight > pluginHeight {
	//			depHeight = pluginHeight
	//		}
	//	}
	//}

	tipHeight, err := util.GetIndexHeight(b.kvStore, b.Name())
	if err != nil {
		return 0, err
	}

	return tipHeight, nil
}

func (b *blockIndexer) DeleteTipBlock(ctx context.Context, blk *block.Block) error {
	return errors.New(fmt.Sprintf("%s indexer not support DeleteTipBlock", b.Name()))
}
