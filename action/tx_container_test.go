// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

func TestTxContainerOversizedSignatureValues(t *testing.T) {
	r := require.New(t)
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	oversized := new(big.Int).Lsh(big.NewInt(1), 256) // 33 bytes
	for _, c := range []struct {
		name  string
		inner func(r, s *big.Int) types.TxData
	}{
		{"legacy", func(r, s *big.Int) types.TxData {
			return &types.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0), V: big.NewInt(27), R: r, S: s}
		}},
		{"dynamic fee", func(r, s *big.Int) types.TxData {
			return &types.DynamicFeeTx{ChainID: big.NewInt(4689), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0), V: big.NewInt(0), R: r, S: s}
		}},
	} {
		for _, sig := range []struct {
			name string
			r, s *big.Int
		}{
			{"R", oversized, big.NewInt(1)},
			{"S", big.NewInt(1), oversized},
		} {
			t.Run(c.name+" oversized "+sig.name, func(t *testing.T) {
				raw, err := types.NewTx(c.inner(sig.r, sig.s)).MarshalBinary()
				r.NoError(err)
				tx := types.Transaction{}
				r.NoError(tx.UnmarshalBinary(raw))
				_, gotR, gotS := tx.RawSignatureValues()
				r.True(gotR.BitLen() > 256 || gotS.BitLen() > 256, "RLP decoding should preserve the oversized value")

				_, _, _, err = ExtractTypeSigPubkey(&tx)
				r.ErrorIs(err, ErrNotSupported)

				err = (&txContainer{}).LoadProto(&iotextypes.ActionCore{
					ChainID: 4689,
					Action:  &iotextypes.ActionCore_TxContainer{TxContainer: &iotextypes.TxContainer{Raw: raw}},
				})
				r.ErrorIs(err, ErrInvalidAct)
			})
		}
	}
}

func TestTxContainerCostIncludesBlobFee(t *testing.T) {
	r := require.New(t)
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tx := types.NewTx(&types.BlobTx{
		ChainID:    uint256.NewInt(4689),
		Nonce:      1,
		GasTipCap:  uint256.NewInt(1),
		GasFeeCap:  uint256.NewInt(10),
		Gas:        21000,
		To:         to,
		Value:      uint256.NewInt(5),
		BlobFeeCap: uint256.NewInt(3),
		BlobHashes: []common.Hash{{1}, {2}},
	})
	cost, err := (&txContainer{tx: tx}).Cost()
	r.NoError(err)
	want := new(big.Int).SetUint64(21000*10 + 5 + 3*2*params.BlobTxBlobGasPerBlob)
	r.Equal(want, cost)

	// no blob fee for other tx types
	tx = types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(4689), Nonce: 1, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(10), Gas: 21000, To: &to, Value: big.NewInt(5)})
	cost, err = (&txContainer{tx: tx}).Cost()
	r.NoError(err)
	r.Equal(big.NewInt(21000*10+5), cost)
}
