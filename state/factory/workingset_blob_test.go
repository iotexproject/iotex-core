// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestWorkingSet_HandleBlob_BlobGasPrice checks which excess blob gas the blob
// fee is priced from. Before the next hardfork it is the parent block's; from
// the next hardfork it is the block's own, which is what the fee cap check and
// the BLOBBASEFEE opcode use.
func TestWorkingSet_HandleBlob_BlobGasPrice(t *testing.T) {
	const (
		height = uint64(10)
		// far enough apart that the two blob prices differ
		parentExcessBlobGas = uint64(0)
		blockExcessBlobGas  = uint64(10 * 3338477)
	)
	parentPrice := protocol.CalcBlobFee(parentExcessBlobGas)
	blockPrice := protocol.CalcBlobFee(blockExcessBlobGas)
	require.NotEqual(t, parentPrice, blockPrice)

	selp, err := action.Sign(action.NewEnvelope(
		action.NewBlobTx(1, 0, 100_000, big.NewInt(1), big.NewInt(1), nil,
			action.NewBlobTxData(uint256.MustFromBig(blockPrice), []common.Hash{{1}}, nil)),
		action.NewTransfer(big.NewInt(1), identityset.Address(29).String(), nil),
	), identityset.PrivateKey(28))
	require.NoError(t, err)

	for _, tt := range []struct {
		name       string
		gateHeight uint64
		want       *big.Int
	}{
		{"before the next hardfork", math.MaxUint64, parentPrice},
		{"from the next hardfork", height, blockPrice},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ws := newStateDBWorkingSet(t)
			g := genesis.TestDefault()
			g.ToBeEnabledBlockHeight = tt.gateHeight
			ctx := genesis.WithGenesisContext(context.Background(), g)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:   height,
				ExcessBlobGas: blockExcessBlobGas,
			})
			ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
				Tip: protocol.TipInfo{
					Height:        height - 1,
					ExcessBlobGas: parentExcessBlobGas,
				},
			})
			ctx = protocol.WithFeatureCtx(ctx)

			receipt := &action.Receipt{}
			r.NoError(ws.handleBlob(ctx, selp, receipt))
			r.Equal(selp.BlobGas(), receipt.BlobGasUsed)
			r.Equal(tt.want, receipt.BlobGasPrice)
		})
	}
}
