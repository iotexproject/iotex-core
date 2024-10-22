// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var _ TxCommonInternal = (*AccessListTx)(nil)

// AccessListTx represents EIP-2930 access list transaction
type AccessListTx struct {
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasPrice   *big.Int
	accessList types.AccessList
}

// NewAccessListTx creates a new access list transaction
func NewAccessListTx(chainID uint32, nonce uint64, gasLimit uint64, gasPrice *big.Int, accessList types.AccessList) *AccessListTx {
	return &AccessListTx{
		chainID:    chainID,
		nonce:      nonce,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		accessList: accessList,
	}
}

func (tx *AccessListTx) TxType() uint32 {
	return AccessListTxType
}

func (tx *AccessListTx) ChainID() uint32 {
	return tx.chainID
}

func (tx *AccessListTx) Nonce() uint64 {
	return tx.nonce
}

func (tx *AccessListTx) Gas() uint64 {
	return tx.gasLimit
}

func (tx *AccessListTx) GasPrice() *big.Int {
	return tx.price()
}

func (tx *AccessListTx) EffectiveGasPrice(_ *big.Int) *big.Int {
	return tx.price()
}

func (tx *AccessListTx) price() *big.Int {
	p := &big.Int{}
	if tx.gasPrice == nil {
		return p
	}
	return p.Set(tx.gasPrice)
}

func (tx *AccessListTx) AccessList() types.AccessList {
	return tx.accessList
}

func (tx *AccessListTx) GasTipCap() *big.Int {
	return tx.price()
}

func (tx *AccessListTx) GasFeeCap() *big.Int {
	return tx.price()
}

func (tx *AccessListTx) BlobGas() uint64 { return 0 }

func (tx *AccessListTx) BlobGasFeeCap() *big.Int { return nil }

func (tx *AccessListTx) BlobHashes() []common.Hash { return nil }

func (tx *AccessListTx) BlobTxSidecar() *types.BlobTxSidecar { return nil }

func (tx *AccessListTx) SanityCheck() error {
	if tx.gasPrice == nil {
		return ErrMissRequiredField
	}
	if tx.gasPrice.Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}

func (tx *AccessListTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		TxType:   AccessListTxType,
		Nonce:    tx.nonce,
		GasLimit: tx.gasLimit,
		ChainID:  tx.chainID,
	}
	if tx.gasPrice != nil {
		actCore.GasPrice = tx.gasPrice.String()
	}
	if len(tx.accessList) > 0 {
		actCore.AccessList = toAccessListProto(tx.accessList)
	}
	return &actCore
}

func toAccessListProto(list types.AccessList) []*iotextypes.AccessTuple {
	if len(list) == 0 {
		return nil
	}
	proto := make([]*iotextypes.AccessTuple, len(list))
	for i, v := range list {
		proto[i] = &iotextypes.AccessTuple{
			Address: hex.EncodeToString(v.Address.Bytes()),
		}
		if numKey := len(v.StorageKeys); numKey > 0 {
			proto[i].StorageKeys = make([]string, numKey)
			for j, key := range v.StorageKeys {
				proto[i].StorageKeys[j] = hex.EncodeToString(key.Bytes())
			}
		}
	}
	return proto
}

func fromAccessListProto(list []*iotextypes.AccessTuple) types.AccessList {
	if len(list) == 0 {
		return nil
	}
	accessList := make(types.AccessList, len(list))
	for i, v := range list {
		accessList[i].Address = common.HexToAddress(v.Address)
		if numKey := len(v.StorageKeys); numKey > 0 {
			accessList[i].StorageKeys = make([]common.Hash, numKey)
			for j, key := range v.StorageKeys {
				accessList[i].StorageKeys[j] = common.HexToHash(key)
			}
		} else {
			accessList[i].StorageKeys = []common.Hash{}
		}
	}
	return accessList
}

func (tx *AccessListTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != AccessListTxType {
		return errors.Wrapf(ErrInvalidProto, "wrong tx type = %d", pb.TxType)
	}
	var gasPrice big.Int
	if price := pb.GetGasPrice(); len(price) > 0 {
		if _, ok := gasPrice.SetString(price, 10); !ok {
			return errors.Errorf("invalid gasPrice %s", price)
		}
	}
	tx.chainID = pb.GetChainID()
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.gasPrice = &gasPrice
	tx.accessList = fromAccessListProto(pb.GetAccessList())
	return nil
}

func (tx *AccessListTx) setNonce(n uint64) {
	tx.nonce = n
}

func (tx *AccessListTx) setGas(gas uint64) {
	tx.gasLimit = gas
}

func (tx *AccessListTx) setChainID(n uint32) {
	tx.chainID = n
}

func (tx *AccessListTx) toEthTx(to *common.Address, value *big.Int, data []byte) *types.Transaction {
	return types.NewTx(&types.AccessListTx{
		Nonce:      tx.nonce,
		GasPrice:   tx.price(),
		Gas:        tx.gasLimit,
		To:         to,
		Value:      value,
		Data:       data,
		AccessList: tx.accessList,
	})
}
