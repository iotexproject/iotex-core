// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import "github.com/iotexproject/iotex-core/blockchain/block"

// BlockCreationSubscriber is an interface which will get notified when a block is created
type BlockCreationSubscriber interface {
	HandleBlock(*block.Block) error
}
