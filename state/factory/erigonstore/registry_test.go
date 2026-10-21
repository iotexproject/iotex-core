// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package erigonstore

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
)

func TestBucketGuardObjectStorageRegistered(t *testing.T) {
	r := require.New(t)
	index, ok := storageRegistry.matchContractIndex(state.StakingNamespace, &staking.BucketGuard{})
	r.True(ok)
	r.Equal(BucketPoolContractIndex, index)
}
