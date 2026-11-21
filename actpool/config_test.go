// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func TestDefaultConfigBlackList(t *testing.T) {
	r := require.New(t)
	r.Equal(genesis.MainnetSetCodeAuthorityBlackList(), DefaultConfig.BlackList)
	r.Equal(genesis.MainnetSetCodeAuthorityBlackListActiveHeight, DefaultConfig.BlackListActiveHeight)
	r.Equal(genesis.MainnetSetCodeAuthorityBlackListRemoval(), DefaultConfig.BlackListRemoval)
}
