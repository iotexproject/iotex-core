// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

func calculateTxRoot(acts []action.SealedEnvelope) hash.Hash256 {
	h := make([]hash.Hash256, 0, len(acts))
	for _, act := range acts {
		h = append(h, act.Hash())
	}
	if len(h) == 0 {
		return hash.ZeroHash256
	}
	return crypto.NewMerkleTree(h).HashTree()
}
