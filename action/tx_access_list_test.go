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

func TestAccessListTx(t *testing.T) {
	r := require.New(t)
	t.Run("proto", func(t *testing.T) {
		tx := &AccessListTx{
			chainID:  3,
			nonce:    8,
			gasLimit: 1001,
			gasPrice: big.NewInt(13),
			accessList: types.AccessList{
				{Address: common.Address{}, StorageKeys: nil},
				{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
				{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
			},
		}
		r.EqualValues(AccessListTxType, tx.Version())
		r.EqualValues(8, tx.Nonce())
		r.EqualValues(1001, tx.Gas())
		r.Equal(big.NewInt(13), tx.GasPrice())
		r.Equal(big.NewInt(13), tx.GasTipCap())
		r.Equal(big.NewInt(13), tx.GasFeeCap())
		r.Equal(3, len(tx.AccessList()))
		r.Zero(tx.BlobGas())
		r.Nil(tx.BlobGasFeeCap())
		r.Nil(tx.BlobHashes())
		r.Nil(tx.BlobTxSidecar())
		b := MustNoErrorV(proto.Marshal(tx.toProto()))
		r.Equal("0802100818e9072202313328034a2a0a28303030303030303030303030303030303030303030303030303030303030303030303030303030304af0010a28303166633234363633333437306366363261653261393536643231653864343831633361363965311240303265393430646430666435623564663463666238643662636439633734656334333365396135633231616362373263626362356265396537313162363738661240303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030301240613631386561356234383965636134326633333161626362303833393466353831663265396461383963386565376537326337343732303438343261626538624ab2020a2833343730636636326165326139353664333864343831633361363965313231653031666332343636124065373730396161376161313631323436363734393139623266303239396539356362623663353438326535633334386431326466653232366637316636336436124061363138656135623438396563613432663333316162636230383339346635383166326539646138396338656537653732633734373230343834326162653862124038383164336264663265313362366538623664363835643232373761343866663337313431343935646464346533643732383966636661353537306632396631124030326539343064643066643562356466346366623864366263643963373465633433336539613563323161636237326362636235626539653731316236373866", hex.EncodeToString(b))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		r.Equal(tx, MustNoErrorV(fromProtoAccessListTx(&pb)))
	})
	ab := AbstractAction{
		version:   AccessListTxType,
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
	expect := &AccessListTx{
		chainID:  3,
		nonce:    8,
		gasLimit: 1001,
		accessList: types.AccessList{
			{Address: common.Address{}, StorageKeys: nil},
			{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
			{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
		},
	}
	t.Run("convert", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			ab.gasPrice = price
			tx := ab.convertToTx()
			acl, ok := tx.(*AccessListTx)
			r.True(ok)
			if price == nil {
				expect.gasPrice = new(big.Int)
			} else {
				expect.gasPrice = new(big.Int).Set(price)
			}
			r.EqualValues(AccessListTxType, acl.Version())
			r.Equal(expect, acl)
		}
	})
	t.Run("loadProtoTxCommon", func(t *testing.T) {
		for _, price := range []*big.Int{
			nil, big.NewInt(13),
		} {
			expect.gasPrice = price
			elp := envelope{}
			r.NoError(elp.loadProtoTxCommon(expect.toProto()))
			acl, ok := elp.common.(*AccessListTx)
			r.True(ok)
			r.EqualValues(AccessListTxType, acl.Version())
			if price == nil {
				expect.gasPrice = new(big.Int)
			}
			r.Equal(expect, acl)
		}
	})
}
