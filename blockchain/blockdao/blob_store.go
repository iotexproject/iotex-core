// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	BlobStore interface {
		Start(context.Context) error
		Stop(context.Context) error
		BlobExpired(uint64, uint64) bool
		GetBlob(hash.Hash256) (*types.BlobTxSidecar, string, error)
		GetBlobsByHeight(uint64) ([]*types.BlobTxSidecar, []string, error)
		PutBlock(*block.Block) error
	}

	// storage for past N-day's blobs, structured as blow:
	// 1. Storage is divided by hour, with each hour's data stored in a bucket.
	//    By default N = 18 so we have 432 buckets, plus 1 extra bucket. At each
	//    new hour, the oldest bucket is found and all data inside it deleted
	//    (so we still have 18 days after deletion), and blobs in this hour are
	//    stored into it
	// 2. Inside each bucket, block height is used as the key to store the blobs
	//    in this block. Maximum number of blobs is 6 for each block, and maximum
	//    data size in the bucket is 131kB x 6 x 720 = 566MB. Maximum total size
	//    of the entire storage is 566MB x 433 = 245GB.
	// 3. A mapping of action hash --> (bucket, block) is maintained in a separate
	//    bucket as the storage index. It converts the requested blob hash to the
	//    bucket and block number, and retrieves the blob object from storage
)
