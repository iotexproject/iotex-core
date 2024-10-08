// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	. "github.com/iotexproject/iotex-core/pkg/util/assertions"
)

func TestLegacyTx(t *testing.T) {
	r := require.New(t)
	t.Run("proto", func(t *testing.T) {
		tx := &LegacyTx{
			chainID:  3,
			nonce:    8,
			gasLimit: 1001,
			gasPrice: big.NewInt(13),
		}
		r.EqualValues(LegacyTxType, tx.Version())
		r.EqualValues(3, tx.ChainID())
		r.EqualValues(8, tx.Nonce())
		r.EqualValues(1001, tx.Gas())
		r.Equal(big.NewInt(13), tx.GasPrice())
		r.Equal(big.NewInt(13), tx.GasTipCap())
		r.Equal(big.NewInt(13), tx.GasFeeCap())
		r.Nil(tx.AccessList())
		r.Zero(tx.BlobGas())
		r.Nil(tx.BlobGasFeeCap())
		r.Nil(tx.BlobHashes())
		r.Nil(tx.BlobTxSidecar())
		b := MustNoErrorV(proto.Marshal(tx.toProto()))
		r.Equal("0801100818e907220231332803", hex.EncodeToString(b))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		tx1 := &LegacyTx{
			chainID:  88,
			nonce:    33,
			gasLimit: 22,
			gasPrice: big.NewInt(5),
		}
		r.NoError(tx1.fromProto(&pb))
		r.Equal(tx, tx1)
	})
	ab := AbstractAction{
		version:   LegacyTxType,
		chainID:   3,
		nonce:     8,
		gasLimit:  1001,
		gasTipCap: big.NewInt(10),
		gasFeeCap: big.NewInt(30),
		accessList: types.AccessList{
			{Address: common.Address{}, StorageKeys: nil},
			{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
			{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
		},
	}
	expect := &LegacyTx{
		chainID:  3,
		nonce:    8,
		gasLimit: 1001,
	}
	t.Run("convert", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			ab.gasPrice = price
			tx := ab.convertToTx()
			legacy, ok := tx.(*LegacyTx)
			r.True(ok)
			if price == nil {
				expect.gasPrice = new(big.Int)
			} else {
				expect.gasPrice = new(big.Int).Set(price)
			}
			r.EqualValues(LegacyTxType, legacy.Version())
			r.Equal(expect, legacy)
		}
	})
	t.Run("loadProtoTxCommon", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			expect.gasPrice = price
			elp := envelope{}
			r.NoError(elp.loadProtoTxCommon(expect.toProto()))
			legacy, ok := elp.common.(*LegacyTx)
			r.True(ok)
			r.EqualValues(LegacyTxType, legacy.Version())
			if price == nil {
				expect.gasPrice = new(big.Int)
			}
			r.Equal(expect, legacy)
		}
	})
}
