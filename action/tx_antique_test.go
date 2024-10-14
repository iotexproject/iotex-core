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

func TestAntiqueTx(t *testing.T) {
	r := require.New(t)
	t.Run("proto", func(t *testing.T) {
		tx := &AntiqueTx{
			LegacyTx: LegacyTx{
				chainID:  8,
				nonce:    3,
				gasLimit: 2024,
				gasPrice: big.NewInt(31),
			},
			version: AntiqueTxType,
		}
		r.EqualValues(AntiqueTxType, tx.Version())
		r.EqualValues(8, tx.ChainID())
		r.EqualValues(3, tx.Nonce())
		r.EqualValues(2024, tx.Gas())
		r.Equal(big.NewInt(31), tx.GasPrice())
		r.Equal(big.NewInt(31), tx.GasTipCap())
		r.Equal(big.NewInt(31), tx.GasFeeCap())
		r.Nil(tx.AccessList())
		r.Zero(tx.BlobGas())
		r.Nil(tx.BlobGasFeeCap())
		r.Nil(tx.BlobHashes())
		r.Nil(tx.BlobTxSidecar())
		b := MustNoErrorV(proto.Marshal(tx.toProto()))
		r.Equal("100318e80f220233312808", hex.EncodeToString(b))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		tx1 := &AntiqueTx{
			LegacyTx: LegacyTx{
				chainID:  88,
				nonce:    33,
				gasLimit: 22,
				gasPrice: big.NewInt(5),
			},
			version: 355,
		}
		r.NoError(tx1.fromProto(&pb))
		r.Equal(tx, tx1)
	})
	ab := AbstractAction{
		version:   _outOfBandTxType18879571,
		chainID:   8,
		nonce:     3,
		gasLimit:  2024,
		gasTipCap: big.NewInt(30),
		gasFeeCap: big.NewInt(10),
		accessList: types.AccessList{
			{Address: common.Address{}, StorageKeys: nil},
			{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
			{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
		},
	}
	expect := &AntiqueTx{
		LegacyTx: LegacyTx{
			chainID:  8,
			nonce:    3,
			gasLimit: 2024,
		},
		version: _outOfBandTxType18879571,
	}
	t.Run("convert", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			ab.gasPrice = price
			tx := ab.convertToTx()
			antique, ok := tx.(*AntiqueTx)
			r.True(ok)
			if price == nil {
				expect.gasPrice = new(big.Int)
			} else {
				expect.gasPrice = new(big.Int).Set(price)
			}
			r.EqualValues(_outOfBandTxType18879571, antique.Version())
			r.Equal(expect, antique)
		}
	})
	t.Run("loadProtoTxCommon", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			expect.gasPrice = price
			elp := envelope{}
			r.NoError(elp.loadProtoTxCommon(expect.toProto()))
			antique, ok := elp.common.(*AntiqueTx)
			r.True(ok)
			r.EqualValues(_outOfBandTxType18879571, antique.Version())
			if price == nil {
				expect.gasPrice = new(big.Int)
			}
			r.Equal(expect, antique)
		}
	})
}
