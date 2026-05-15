package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// errs for SetCodeTx
var (
	ErrEmptyAuthList   = errors.New("EIP-7702 transaction with empty auth list")
	ErrSetCodeTxCreate = errors.New("EIP-7702 transaction cannot be used to create contract")
)

// SetCodeTx implements the EIP-7702 transaction type which temporarily installs
// the code at the signer's address.
type SetCodeTx struct {
	DynamicFeeTx
	authList []types.SetCodeAuthorization
}

// TxType returns the transaction type
func (tx *SetCodeTx) TxType() uint32 {
	return SetCodeTxType
}

// SetCodeAuthorizations returns the set code authorizations
func (tx *SetCodeTx) SetCodeAuthorizations() []types.SetCodeAuthorization {
	return tx.authList
}

func (tx *SetCodeTx) toProto() *iotextypes.ActionCore {
	actCore := &iotextypes.ActionCore{
		TxType:   SetCodeTxType,
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
	authListProto := make([]*iotextypes.SetCodeAuthorization, 0, len(tx.authList))
	for _, auth := range tx.authList {
		authProto := &iotextypes.SetCodeAuthorization{
			ChainID: uint32(auth.ChainID.Uint64()),
			Address: auth.Address.Bytes(),
			Nonce:   auth.Nonce,
			V:       uint64(auth.V),
			R:       auth.R.Bytes(),
			S:       auth.S.Bytes(),
		}
		authListProto = append(authListProto, authProto)
	}
	actCore.SetCodeAuthList = authListProto
	return actCore
}

func (tx *SetCodeTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != SetCodeTxType {
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
	for _, authProto := range pb.GetSetCodeAuthList() {
		auth := types.SetCodeAuthorization{
			ChainID: *uint256.NewInt(uint64(authProto.ChainID)),
			Address: common.BytesToAddress(authProto.Address),
			Nonce:   authProto.Nonce,
			V:       uint8(authProto.V),
			R:       *uint256.NewInt(0).SetBytes(authProto.R),
			S:       *uint256.NewInt(0).SetBytes(authProto.S),
		}
		tx.authList = append(tx.authList, auth)
	}
	return nil
}

func (tx *SetCodeTx) toEthTx(to *common.Address, value *big.Int, data []byte) *types.Transaction {
	return types.NewTx(&types.SetCodeTx{
		Nonce:      tx.nonce,
		GasTipCap:  uint256.MustFromBig(tx.GasTipCap()),
		GasFeeCap:  uint256.MustFromBig(tx.GasFeeCap()),
		Gas:        tx.gasLimit,
		To:         *to,
		Value:      uint256.MustFromBig(value),
		Data:       data,
		AccessList: tx.accessList,
		AuthList:   tx.authList,
	})
}
