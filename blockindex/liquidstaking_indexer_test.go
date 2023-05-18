// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestContractStakingIndexerLoadCache(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
	r.NoError(indexer.Start(context.Background()))

	// create a stake
	dirty := newContractStakingDirty(indexer.cache)
	height := uint64(1)
	dirty.putHeight(height)
	err = dirty.handleBucketTypeActivatedEvent(eventParam{
		"amount":   big.NewInt(10),
		"duration": big.NewInt(100),
	}, height)
	r.NoError(err)
	owner := identityset.Address(0)

	err = dirty.handleTransferEvent(eventParam{
		"to":      common.BytesToAddress(owner.Bytes()),
		"tokenId": big.NewInt(1),
	})
	r.NoError(err)
	delegate := identityset.Address(1)
	err = dirty.handleStakedEvent(eventParam{
		"tokenId":  big.NewInt(1),
		"delegate": common.BytesToAddress(delegate.Bytes()),
		"amount":   big.NewInt(10),
		"duration": big.NewInt(100),
	}, height)
	r.NoError(err)
	err = indexer.commit(dirty)
	r.NoError(err)
	buckets, err := indexer.Buckets()
	r.NoError(err)
	r.NoError(indexer.Stop(context.Background()))

	// load cache from db
	newIndexer := &contractStakingIndexer{
		kvstore: db.NewBoltDB(cfg),
		cache:   newContractStakingCache(),
	}
	r.NoError(newIndexer.Start(context.Background()))

	// check cache
	newBuckets, err := newIndexer.Buckets()
	r.NoError(err)
	r.Equal(len(buckets), len(newBuckets))
	for i := range buckets {
		r.EqualValues(buckets[i], newBuckets[i])
	}
	newHeight, err := newIndexer.Height()
	r.NoError(err)
	r.Equal(height, newHeight)
	r.EqualValues(1, newIndexer.TotalBucketCount())
	r.NoError(newIndexer.Stop(context.Background()))
}
