package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestDynamicFeeTx(t *testing.T) {
	r := require.New(t)
	acl := types.AccessList{
		{Address: common.HexToAddress("0x1"), StorageKeys: []common.Hash{common.HexToHash("0x2")}},
	}
	tx := &DynamicFeeTx{
		chainID:    1,
		nonce:      2,
		gasLimit:   3,
		gasTipCap:  big.NewInt(4),
		gasFeeCap:  big.NewInt(5),
		accessList: acl,
	}
	// case: getters
	r.Equal(tx.Version(), uint32(DynamicFeeTxType))
	r.Equal(tx.ChainID(), uint32(1))
	r.Equal(tx.Nonce(), uint64(2))
	r.Equal(tx.Gas(), uint64(3))
	r.Equal(tx.GasTipCap(), big.NewInt(4))
	r.Equal(tx.GasFeeCap(), big.NewInt(5))
	r.Equal(tx.GasPrice(), tx.GasFeeCap())
	r.Equal(tx.AccessList(), acl)
	r.Equal(uint64(0), tx.BlobGas())
	r.Nil(tx.BlobGasFeeCap())
	r.Len(tx.BlobHashes(), 0)
	r.Nil(tx.BlobTxSidecar())
	// case: toProto and fromProto
	core := tx.toProto()
	tx2 := &DynamicFeeTx{}
	r.NoError(tx2.fromProto(core))
	r.Equal(tx, tx2)
	// case: sanity check
	r.NoError(tx.SanityCheck())
	tx.gasTipCap = big.NewInt(-1)
	r.ErrorIs(ErrNegativeValue, tx.SanityCheck())
	tx.gasTipCap = big.NewInt(10)
	r.ErrorIs(tx.SanityCheck(), ErrGasFeeCapLessThanTipCap)
	// case: setters
	tx.setChainID(2)
	r.Equal(uint32(2), tx.ChainID())
	tx.setNonce(3)
	r.Equal(uint64(3), tx.Nonce())
	tx.setGas(4)
	r.Equal(uint64(4), tx.Gas())
}
