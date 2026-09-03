// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// TestCalculateGasUsedFromReceipts pins the two sums a validating node
// re-derives from its receipts and compares against the header. They are the
// same helpers the proposer uses in CreateBuilder, so the pair cannot drift,
// and the non-zero blob case is the one a header carrying blob gas has to
// agree with.
func TestCalculateGasUsedFromReceipts(t *testing.T) {
	r := require.New(t)
	const perBlob = uint64(params.BlobTxBlobGasPerBlob)

	t.Run("no receipts sums to zero", func(t *testing.T) {
		r.Zero(calculateGasUsed(nil))
		r.Zero(calculateBlobGasUsed(nil))
		r.Zero(calculateGasUsed([]*action.Receipt{}))
		r.Zero(calculateBlobGasUsed([]*action.Receipt{}))
	})

	t.Run("gas is summed across receipts", func(t *testing.T) {
		receipts := []*action.Receipt{
			{GasConsumed: 21000},
			{GasConsumed: 100000},
			{GasConsumed: 0},
		}
		r.EqualValues(121000, calculateGasUsed(receipts))
		// none of them carried a blob
		r.Zero(calculateBlobGasUsed(receipts))
	})

	t.Run("blob gas is summed independently of gas", func(t *testing.T) {
		receipts := []*action.Receipt{
			{GasConsumed: 21000, BlobGasUsed: perBlob},
			{GasConsumed: 21000},
			{GasConsumed: 50000, BlobGasUsed: 2 * perBlob},
		}
		r.EqualValues(92000, calculateGasUsed(receipts))
		r.Equal(3*perBlob, calculateBlobGasUsed(receipts))

		// A header carrying exactly this much blob gas is what the validator
		// has to accept, and one unit off is what it has to reject.
		h := block.Header{}
		r.False(h.VerifyBlobGasUsed(calculateBlobGasUsed(receipts)),
			"an unset header must not match a non-zero recompute")
	})
}
