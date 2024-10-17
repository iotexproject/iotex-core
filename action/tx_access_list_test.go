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

	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
)

func TestAccessListTx(t *testing.T) {
	r := require.New(t)
	expect := &AccessListTx{
		chainID:    3,
		nonce:      8,
		gasLimit:   1001,
		gasPrice:   big.NewInt(13),
		accessList: createTestACL(),
	}
	t.Run("proto", func(t *testing.T) {
		r.EqualValues(AccessListTxType, expect.TxType())
		r.EqualValues(8, expect.Nonce())
		r.EqualValues(1001, expect.Gas())
		r.Equal(big.NewInt(13), expect.GasPrice())
		r.Equal(big.NewInt(13), expect.GasTipCap())
		r.Equal(big.NewInt(13), expect.GasFeeCap())
		r.Equal(createTestACL(), expect.AccessList())
		r.Zero(expect.BlobGas())
		r.Nil(expect.BlobGasFeeCap())
		r.Nil(expect.BlobHashes())
		r.Nil(expect.BlobTxSidecar())
		epb := expect.toProto()
		r.Zero(epb.Version)
		b := MustNoErrorV(proto.Marshal(epb))
		r.Equal("100818e9072202313328034a2a0a28303030303030303030303030303030303030303030303030303030303030303030303030303030304af0010a28303166633234363633333437306366363261653261393536643231653864343831633361363965311240303265393430646430666435623564663463666238643662636439633734656334333365396135633231616362373263626362356265396537313162363738661240303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030301240613631386561356234383965636134326633333161626362303833393466353831663265396461383963386565376537326337343732303438343261626538624ab2020a2833343730636636326165326139353664333864343831633361363965313231653031666332343636124065373730396161376161313631323436363734393139623266303239396539356362623663353438326535633334386431326466653232366637316636336436124061363138656135623438396563613432663333316162636230383339346635383166326539646138396338656537653732633734373230343834326162653862124038383164336264663265313362366538623664363835643232373761343866663337313431343935646464346533643732383966636661353537306632396631124030326539343064643066643562356466346366623864366263643963373465633433336539613563323161636237326362636235626539653731316236373866e00101", hex.EncodeToString(b))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		tx1 := &AccessListTx{
			chainID:    88,
			nonce:      33,
			gasLimit:   22,
			gasPrice:   big.NewInt(5),
			accessList: types.AccessList{},
		}
		r.NoError(tx1.fromProto(&pb))
		r.Equal(expect, tx1)
		pb.TxType = DynamicFeeTxType
		r.ErrorIs(tx1.fromProto(&pb), ErrInvalidProto)
	})
	t.Run("sanity", func(t *testing.T) {
		r.NoError(expect.SanityCheck())
		a := expect.gasPrice
		expect.gasPrice = nil
		r.ErrorIs(expect.SanityCheck(), ErrMissRequiredField)
		expect.gasPrice = big.NewInt(-1)
		r.ErrorIs(expect.SanityCheck(), ErrNegativeValue)
		expect.gasPrice = a
	})
	ab := AbstractAction{
		txType:     AccessListTxType,
		chainID:    3,
		nonce:      8,
		gasLimit:   1001,
		gasTipCap:  big.NewInt(10),
		gasFeeCap:  big.NewInt(30),
		accessList: createTestACL(),
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
			r.EqualValues(AccessListTxType, acl.TxType())
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
			r.EqualValues(AccessListTxType, acl.TxType())
			if price == nil {
				expect.gasPrice = new(big.Int)
			}
			r.Equal(expect, acl)
		}
	})
}

func createTestACL() types.AccessList {
	return types.AccessList{
		{Address: common.Address{}, StorageKeys: []common.Hash{}},
		{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
		{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
	}
}
