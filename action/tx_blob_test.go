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
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	. "github.com/iotexproject/iotex-core/pkg/util/assertions"
)

func TestBlobTx(t *testing.T) {
	r := require.New(t)
	testACL := types.AccessList{
		{Address: common.Address{}, StorageKeys: nil},
		{Address: _c1, StorageKeys: []common.Hash{_k1, {}, _k3}},
		{Address: _c2, StorageKeys: []common.Hash{_k2, _k3, _k4, _k1}},
	}
	testBlob := createTestBlobTxData()
	expect := &BlobTx{
		chainID:    3,
		nonce:      8,
		gasLimit:   1001,
		gasTipCap:  uint256.NewInt(13),
		gasFeeCap:  uint256.NewInt(27),
		accessList: testACL,
		blob:       testBlob,
	}
	t.Run("proto", func(t *testing.T) {
		r.EqualValues(BlobTxType, expect.Version())
		r.EqualValues(8, expect.Nonce())
		r.EqualValues(1001, expect.Gas())
		r.Equal(big.NewInt(27), expect.GasPrice())
		r.Equal(big.NewInt(13), expect.GasTipCap())
		r.Equal(big.NewInt(27), expect.GasFeeCap())
		r.Equal(testACL, expect.AccessList())
		r.EqualValues(131072, expect.BlobGas())
		r.Equal(big.NewInt(15), expect.BlobGasFeeCap())
		r.Equal(testBlob.hashes(), expect.BlobHashes())
		r.Equal(testBlob.sidecar, expect.BlobTxSidecar())
		b := MustNoErrorV(proto.Marshal(expect.toProto()))
		h := hash.Hash256b(b[:])
		r.Equal("0e6639e7bd18c71233cd064704eddb4819f767af50610aa159899a1d717e5099", hex.EncodeToString(h[:]))
		pb := iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(b, &pb))
		tx1 := &BlobTx{}
		r.NoError(tx1.fromProto(&pb))
		r.Equal(expect, tx1)
	})
	t.Run("sanity", func(t *testing.T) {
		r.NoError(expect.SanityCheck())
		a := expect.gasTipCap
		expect.gasTipCap = nil
		r.ErrorIs(expect.SanityCheck(), ErrMissRequiredField)
		expect.gasTipCap = uint256.NewInt(1)
		expect.gasTipCap.Lsh(expect.gasTipCap, 255)
		r.ErrorIs(expect.SanityCheck(), ErrNegativeValue)
		expect.gasTipCap = uint256.NewInt(28)
		r.ErrorIs(expect.SanityCheck(), ErrGasTipOverFeeCap)
		expect.gasTipCap = a
		r.NoError(expect.SanityCheck())
		b := expect.gasFeeCap
		expect.gasFeeCap = nil
		r.ErrorIs(expect.SanityCheck(), ErrMissRequiredField)
		expect.gasFeeCap = uint256.NewInt(1)
		expect.gasFeeCap.Lsh(expect.gasFeeCap, 255)
		r.ErrorIs(expect.SanityCheck(), ErrNegativeValue)
		expect.gasFeeCap = uint256.NewInt(12)
		r.ErrorIs(expect.SanityCheck(), ErrGasTipOverFeeCap)
		expect.gasTipCap = a
		expect.gasFeeCap = b
	})
	t.Run("loadProtoTxCommon", func(t *testing.T) {
		elp := envelope{}
		r.NoError(elp.loadProtoTxCommon(expect.toProto()))
		blob, ok := elp.common.(*BlobTx)
		r.True(ok)
		r.EqualValues(BlobTxType, blob.Version())
		r.Equal(expect, blob)
	})
	t.Run("build from setter", func(t *testing.T) {
		tx := (&EnvelopeBuilder{}).SetVersion(BlobTxType).SetChainID(expect.ChainID()).SetNonce(expect.Nonce()).
			SetGasLimit(expect.Gas()).SetDynamicGas(expect.GasFeeCap(), expect.GasTipCap()).
			SetAccessList(testACL).SetBlobTxData(testBlob.blobFeeCap, testBlob.blobHashes, testBlob.sidecar).
			SetAction(&Transfer{}).Build()
		blob, ok := tx.(*envelope).common.(*BlobTx)
		r.True(ok)
		r.EqualValues(BlobTxType, blob.Version())
		r.Equal(expect, blob)
	})
	t.Run("build from EthTx", func(t *testing.T) {
		ethTx := types.NewTx(&types.BlobTx{
			Nonce:      expect.Nonce(),
			Gas:        expect.Gas(),
			GasTipCap:  uint256.MustFromBig(expect.GasTipCap()),
			GasFeeCap:  uint256.MustFromBig(expect.GasFeeCap()),
			AccessList: expect.AccessList(),
			BlobFeeCap: testBlob.gasFeeCap(),
			BlobHashes: testBlob.hashes(),
			Sidecar:    testBlob.sidecar,
			Value:      uint256.NewInt(13),
			To:         common.BytesToAddress([]byte{}),
			Data:       []byte{1, 2, 3},
		})
		tx, err := (&EnvelopeBuilder{}).SetChainID(expect.ChainID()).BuildTransfer(ethTx)
		r.NoError(err)
		blob, ok := tx.(*envelope).common.(*BlobTx)
		r.True(ok)
		r.EqualValues(BlobTxType, blob.Version())
		r.Equal(expect, blob)
		tsf, ok := tx.(*envelope).Action().(*Transfer)
		r.True(ok)
		to := MustNoErrorV(address.FromBytes(ethTx.To()[:]))
		r.Equal(NewTransfer(ethTx.Value(), to.String(), ethTx.Data()), tsf)
		tx2 := &envelope{}
		r.NoError(tx2.LoadProto(tx.Proto()))
		r.Equal(tx, tx2)
	})
}
