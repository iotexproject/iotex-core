// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

// Minter is the interface of block minter
type Minter interface {
	// NewBlockBuilder creates block builder
	NewBlockBuilder(context.Context, map[string][]action.SealedEnvelope, []action.SealedEnvelope) (*block.Builder, error)
}
