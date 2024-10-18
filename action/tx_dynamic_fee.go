package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var _ TxCommonInternal = (*DynamicFeeTx)(nil)

// DynamicFeeTx is a transaction type with dynamic fee cap and tip cap
type DynamicFeeTx struct {
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasTipCap  *big.Int
	gasFeeCap  *big.Int
	accessList types.AccessList
}

// NewDynamicFeeTx creates a new dynamic fee transaction
func NewDynamicFeeTx(chainID uint32, nonce uint64, gasLimit uint64, gasFeeCap, gasTipCap *big.Int, accessList types.AccessList) *DynamicFeeTx {
	return &DynamicFeeTx{
		chainID:    chainID,
		nonce:      nonce,
		gasLimit:   gasLimit,
		gasFeeCap:  gasFeeCap,
		gasTipCap:  gasTipCap,
		accessList: accessList,
	}
}

func (tx *DynamicFeeTx) TxType() uint32 {
	return DynamicFeeTxType
}

func (tx *DynamicFeeTx) ChainID() uint32 {
	return tx.chainID
}

func (tx *DynamicFeeTx) Nonce() uint64 {
	return tx.nonce
}

func (tx *DynamicFeeTx) Gas() uint64 {
	return tx.gasLimit
}

func (tx *DynamicFeeTx) GasTipCap() *big.Int {
	v := big.NewInt(0)
	if tx.gasTipCap != nil {
		v.Set(tx.gasTipCap)
	}
	return v
}

func (tx *DynamicFeeTx) GasFeeCap() *big.Int {
	v := big.NewInt(0)
	if tx.gasFeeCap != nil {
		v.Set(tx.gasFeeCap)
	}
	return v
}

func (tx *DynamicFeeTx) EffectiveGasPrice(baseFee *big.Int) *big.Int {
	tip := tx.GasFeeCap()
	if baseFee == nil {
		return tip
	}
	tip.Sub(tip, baseFee)
	if tipCap := tx.GasTipCap(); tip.Cmp(tipCap) > 0 {
		tip.Set(tipCap)
	}
	return tip.Add(tip, baseFee)
}

func (tx *DynamicFeeTx) AccessList() types.AccessList {
	return tx.accessList
}

func (tx *DynamicFeeTx) GasPrice() *big.Int {
	return tx.GasFeeCap()
}

func (tx *DynamicFeeTx) BlobGas() uint64 { return 0 }

func (tx *DynamicFeeTx) BlobGasFeeCap() *big.Int { return nil }

func (tx *DynamicFeeTx) BlobHashes() []common.Hash { return nil }

func (tx *DynamicFeeTx) BlobTxSidecar() *types.BlobTxSidecar { return nil }

func (tx *DynamicFeeTx) SanityCheck() error {
	if tx.gasTipCap == nil || tx.gasFeeCap == nil {
		return ErrMissRequiredField
	}
	if tx.gasTipCap.Sign() < 0 {
		return ErrNegativeValue
	}
	if tx.gasFeeCap.Sign() < 0 {
		return ErrNegativeValue
	}
	if tx.gasFeeCap.Cmp(tx.gasTipCap) < 0 {
		return ErrGasTipOverFeeCap
	}
	if tx.gasFeeCap.BitLen() > 256 {
		return errors.Wrap(ErrValueVeryHigh, "fee cap is too high")
	}
	if tx.gasTipCap.BitLen() > 256 {
		return errors.Wrap(ErrValueVeryHigh, "tip cap is too high")
	}
	return nil
}

func (tx *DynamicFeeTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		TxType:   DynamicFeeTxType,
		Nonce:    tx.nonce,
		GasLimit: tx.gasLimit,
		ChainID:  tx.chainID,
	}
	if tx.gasFeeCap != nil {
		actCore.GasFeeCap = tx.gasFeeCap.String()
	}
	if tx.gasTipCap != nil {
		actCore.GasTipCap = tx.gasTipCap.String()
	}
	if len(tx.accessList) > 0 {
		actCore.AccessList = toAccessListProto(tx.accessList)
	}
	return &actCore
}

func (tx *DynamicFeeTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != DynamicFeeTxType {
		return errors.Wrapf(ErrInvalidProto, "wrong tx type = %d", pb.TxType)
	}
	var (
		feeCap *big.Int
		tipCap *big.Int
	)
	if feeCapStr := pb.GetGasFeeCap(); len(feeCapStr) > 0 {
		v, ok := big.NewInt(0).SetString(feeCapStr, 10)
		if !ok {
			return errors.Errorf("invalid feeCap %s", feeCapStr)
		}
		feeCap = v
	}
	if tipCapStr := pb.GetGasTipCap(); len(tipCapStr) > 0 {
		v, ok := big.NewInt(0).SetString(tipCapStr, 10)
		if !ok {
			return errors.Errorf("invalid tipCap %s", tipCapStr)
		}
		tipCap = v
	}
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.chainID = pb.GetChainID()
	tx.gasFeeCap = feeCap
	tx.gasTipCap = tipCap
	tx.accessList = fromAccessListProto(pb.GetAccessList())
	return nil
}

func (tx *DynamicFeeTx) setNonce(n uint64) {
	tx.nonce = n
}

func (tx *DynamicFeeTx) setGas(gas uint64) {
	tx.gasLimit = gas
}

func (tx *DynamicFeeTx) setChainID(n uint32) {
	tx.chainID = n
}

func (tx *DynamicFeeTx) toEthTx(to *common.Address, value *big.Int, data []byte) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		Nonce:      tx.nonce,
		GasTipCap:  tx.GasTipCap(),
		GasFeeCap:  tx.GasFeeCap(),
		Gas:        tx.gasLimit,
		To:         to,
		Value:      value,
		Data:       data,
		AccessList: tx.accessList,
	})
}
