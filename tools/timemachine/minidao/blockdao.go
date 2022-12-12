// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minidao

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
)

type blockDAO struct {
	*blockdao.BlockDAODB
}

// NewBlockDAO instantiates a block DAO
func NewBlockDAO(indexers []blockdao.BlockIndexer, cfg db.Config, deser *block.Deserializer) blockdao.BlockDAO {
	return &blockDAO{blockdao.NewBlockDAO(indexers, cfg, deser).(*blockdao.BlockDAODB)}
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	return dao.StartDB(ctx)
}
