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

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestDynamicFeeTxSelf(t *testing.T) {
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
	r.Equal(tx.TxType(), uint32(DynamicFeeTxType))
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
	// case: serialization
	pb := tx.toProto()
	r.Zero(pb.Version)
	b, err := proto.Marshal(pb)
	r.NoError(err)
	r.Equal("1002180328013201343a01354a6c0a2830303030303030303030303030303030303030303030303030303030303030303030303030303031124030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303032e00102", hex.EncodeToString(b))
	core := &iotextypes.ActionCore{}
	r.NoError(proto.Unmarshal(b, core))
	tx2 := &DynamicFeeTx{
		chainID:    88,
		nonce:      33,
		gasLimit:   22,
		gasTipCap:  big.NewInt(5),
		gasFeeCap:  big.NewInt(6),
		accessList: types.AccessList{},
	}
	r.NoError(tx2.fromProto(core))
	r.Equal(tx, tx2)
	core.TxType = BlobTxType
	r.ErrorIs(tx2.fromProto(core), ErrInvalidProto)
	// case: sanity check
	r.NoError(tx.SanityCheck())
	tx.gasTipCap = big.NewInt(-1)
	r.ErrorIs(ErrNegativeValue, tx.SanityCheck())
	tx.gasTipCap = big.NewInt(10)
	r.ErrorIs(tx.SanityCheck(), ErrGasTipOverFeeCap)
	// case: setters
	tx.setChainID(2)
	r.Equal(uint32(2), tx.ChainID())
	tx.setNonce(3)
	r.Equal(uint64(3), tx.Nonce())
	tx.setGas(4)
	r.Equal(uint64(4), tx.Gas())
}

func TestDynamicFeeTxFromEth(t *testing.T) {
	r := require.New(t)
	acl := types.AccessList{
		{Address: common.HexToAddress("0x1"), StorageKeys: []common.Hash{common.HexToHash("0x2")}},
	}
	builder := &EnvelopeBuilder{}
	builder.SetChainID(1)
	to := common.BytesToAddress(identityset.Address(1).Bytes())
	tx, err := builder.BuildTransfer(types.NewTx(&types.DynamicFeeTx{
		Nonce:      2,
		Gas:        3,
		GasTipCap:  big.NewInt(4),
		GasFeeCap:  big.NewInt(5),
		AccessList: acl,
		Value:      big.NewInt(1),
		To:         &to,
	}))
	r.NoError(err)
	r.Equal(&DynamicFeeTx{
		chainID:    1,
		nonce:      2,
		gasLimit:   3,
		gasTipCap:  big.NewInt(4),
		gasFeeCap:  big.NewInt(5),
		accessList: acl,
	}, tx.(*envelope).common)
	tx2 := &envelope{}
	r.NoError(tx2.LoadProto(tx.Proto()))
	r.Equal(tx, tx2)
}
