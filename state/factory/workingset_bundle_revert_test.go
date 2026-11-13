// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestMint_BundleRevertAfterFinalizedAction covers a bundle that fails after
// one of its actions already ran through. On statedb the pre-bundle snapshot
// reverts cleanly, the bundle is dropped and minting continues. On the erigon
// secondary store FinalizeTx has dropped the revert journal, so the snapshot
// can no longer be reverted; the revert returns that as an error, and Mint
// closes the working set so the erigon read transaction is released before
// the factory stops.
func TestMint_BundleRevertAfterFinalizedAction(t *testing.T) {
	for _, withHistory := range []bool{false, true} {
		name := "statedb"
		if withHistory {
			name = "erigon"
		}
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			// sender 26 runs first and succeeds, sender 28 fails in Handle;
			// both are at their first nonce so the continuity gate lets the
			// bundle run and the revert path is exercised.
			proposer, validator, ctx := newPoisonMintEnv(t, &poisonProtocol{stage: failHandle}, withHistory, context.Background())
			ok, poison := makeTransferFrom(t, 26, 1, 1), makeTransferFrom(t, 28, 1, 1)
			ap := bundleActPool(t, ok, poison)

			var (
				blk      *block.Block
				err      error
				panicked any
			)
			func() {
				defer func() { panicked = recover() }()
				blk, err = proposer.Mint(ctx, ap, identityset.PrivateKey(27))
			}()
			require.Nil(panicked, "bundle revert must not panic")
			if withHistory {
				require.Error(err, "erigon proposer must fail the mint rather than emit a half-reverted bundle")
				return
			}
			require.NoError(err)
			require.Empty(blk.Actions, "the failed bundle must be left out")
			require.NoError(validator.Validate(ctx, blk))
		})
	}
}
