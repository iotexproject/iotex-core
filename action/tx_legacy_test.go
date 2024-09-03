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
		b := MustNoErrorV(proto.Marshal(tx.toProto()))
		r.Equal("0801100818e907220231332803", hex.EncodeToString(b))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		r.Equal(tx, MustNoErrorV(fromProtoLegacyTx(&pb)))
	})
	t.Run("convert", func(t *testing.T) {
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
}
