// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

// overflowBig returns a big.Int strictly larger than the uint256 range.
func overflowBig() *big.Int {
	v := new(big.Int).Lsh(big.NewInt(1), 256) // 2^256, one past the max
	return v
}

// TestBlobTx_fromProto_missingBlobData ensures a blob tx proto without the blob
// data payload is rejected instead of leaving a nil blob that later derefs.
func TestBlobTx_fromProto_missingBlobData(t *testing.T) {
	r := require.New(t)
	pb := &iotextypes.ActionCore{
		TxType:    BlobTxType,
		GasFeeCap: "27",
		GasTipCap: "13",
		// BlobTxData deliberately omitted
	}
	tx := &BlobTx{}
	r.ErrorIs(tx.fromProto(pb), ErrMissingBlobData)
}

// TestToEthTx_guards ensures the eth-tx conversion returns an error instead of
// panicking on inputs that would otherwise deref a nil address or overflow a
// uint256 field.
func TestToEthTx_guards(t *testing.T) {
	r := require.New(t)
	addr := common.BytesToAddress([]byte{1})

	t.Run("setcode nil to", func(t *testing.T) {
		tx := &SetCodeTx{}
		_, err := tx.toEthTx(nil, big.NewInt(1), nil)
		r.ErrorIs(err, ErrSetCodeTxCreate)
	})
	t.Run("setcode value overflow", func(t *testing.T) {
		tx := &SetCodeTx{}
		_, err := tx.toEthTx(&addr, overflowBig(), nil)
		r.ErrorIs(err, ErrValueVeryHigh)
	})
	t.Run("setcode ok", func(t *testing.T) {
		tx := &SetCodeTx{}
		ethTx, err := tx.toEthTx(&addr, big.NewInt(1), nil)
		r.NoError(err)
		r.NotNil(ethTx)
	})
	t.Run("blob nil to", func(t *testing.T) {
		tx := &BlobTx{blob: createTestBlobTxData()}
		_, err := tx.toEthTx(nil, big.NewInt(1), nil)
		r.ErrorIs(err, ErrBlobTxCreate)
	})
	t.Run("blob missing blob data", func(t *testing.T) {
		tx := &BlobTx{}
		_, err := tx.toEthTx(&addr, big.NewInt(1), nil)
		r.ErrorIs(err, ErrMissingBlobData)
	})
	t.Run("blob value overflow", func(t *testing.T) {
		tx := &BlobTx{
			gasTipCap: uint256.NewInt(1),
			gasFeeCap: uint256.NewInt(1),
			blob:      createTestBlobTxData(),
		}
		_, err := tx.toEthTx(&addr, overflowBig(), nil)
		r.ErrorIs(err, ErrValueVeryHigh)
	})
}
