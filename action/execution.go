// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

// const
const (
	EmptyAddress                     = ""
	ExecutionDataGas          uint64 = 100   // per-byte execution data gas
	ExecutionBaseIntrinsicGas uint64 = 10000 // base intrinsic gas for execution
	TxAccessListAddressGas    uint64 = 2400  // Per address specified in EIP 2930 access list
	TxAccessListStorageKeyGas uint64 = 1900  // Per storage key specified in EIP 2930 access list
)

var _ hasDestination = (*Execution)(nil)

// Execution defines the struct of account-based contract execution
type Execution struct {
	AbstractAction

	contract   string
	amount     *big.Int
	data       []byte
	accessList types.AccessList
}

// NewExecution returns a Execution instance
func NewExecution(
	contractAddress string,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte,
) (*Execution, error) {
	return &Execution{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		contract: contractAddress,
		amount:   amount,
		data:     data,
	}, nil
}

// Contract returns a contract address
func (ex *Execution) Contract() string { return ex.contract }

// Destination returns a contract address
func (ex *Execution) Destination() string { return ex.contract }

// Recipient is same as Contract()
func (ex *Execution) Recipient() string { return ex.contract }

// Amount returns the amount
func (ex *Execution) Amount() *big.Int { return ex.amount }

// Data returns the data bytes
func (ex *Execution) Data() []byte { return ex.data }

// Payload is same as Data()
func (ex *Execution) Payload() []byte { return ex.data }

// AccessList returns the access list
func (ex *Execution) AccessList() types.AccessList { return ex.accessList }

func toAccessListProto(list types.AccessList) []*iotextypes.AccessTuple {
	if len(list) == 0 {
		return nil
	}
	proto := make([]*iotextypes.AccessTuple, len(list))
	for i, v := range list {
		proto[i] = &iotextypes.AccessTuple{}
		proto[i].Address = hex.EncodeToString(v.Address.Bytes())
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
		}
	}
	return accessList
}

// TotalSize returns the total size of this Execution
func (ex *Execution) TotalSize() uint32 {
	size := ex.BasicActionSize()
	if ex.amount != nil && len(ex.amount.Bytes()) > 0 {
		size += uint32(len(ex.amount.Bytes()))
	}

	return size + uint32(len(ex.data))
}

// Serialize returns a raw byte stream of this Transfer
func (ex *Execution) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ex.Proto()))
}

// Proto converts Execution to protobuf's Execution
func (ex *Execution) Proto() *iotextypes.Execution {
	act := &iotextypes.Execution{
		Contract: ex.contract,
		Data:     ex.data,
	}
	if ex.amount != nil && len(ex.amount.String()) > 0 {
		act.Amount = ex.amount.String()
	}
	act.AccessList = toAccessListProto(ex.accessList)
	return act
}

// LoadProto converts a protobuf's Execution to Execution
func (ex *Execution) LoadProto(pbAct *iotextypes.Execution) error {
	if pbAct == nil {
		return ErrNilProto
	}
	if ex == nil {
		return ErrNilAction
	}
	*ex = Execution{}

	ex.contract = pbAct.GetContract()
	if pbAct.GetAmount() == "" {
		ex.amount = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(pbAct.GetAmount(), 10)
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetAmount())
		}
		ex.amount = amount
	}
	ex.data = pbAct.GetData()
	ex.accessList = fromAccessListProto(pbAct.AccessList)
	return nil
}

// IntrinsicGas returns the intrinsic gas of an execution
func (ex *Execution) IntrinsicGas() (uint64, error) {
	gas, err := CalculateIntrinsicGas(ExecutionBaseIntrinsicGas, ExecutionDataGas, uint64(len(ex.Data())))
	if err != nil {
		return gas, err
	}
	if len(ex.accessList) > 0 {
		gas += uint64(len(ex.accessList)) * TxAccessListAddressGas
		gas += uint64(ex.accessList.StorageKeys()) * TxAccessListStorageKeyGas
	}
	return gas, nil
}

// Cost returns the cost of an execution
func (ex *Execution) Cost() (*big.Int, error) {
	maxExecFee := big.NewInt(0).Mul(ex.GasPrice(), big.NewInt(0).SetUint64(ex.GasLimit()))
	return big.NewInt(0).Add(ex.Amount(), maxExecFee), nil
}

// SanityCheck validates the variables in the action
func (ex *Execution) SanityCheck() error {
	// Reject execution of negative amount
	if ex.Amount().Sign() < 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}
	// check if contract's address is valid
	if ex.Contract() != EmptyAddress {
		if _, err := address.FromString(ex.Contract()); err != nil {
			return errors.Wrapf(err, "error when validating contract's address %s", ex.Contract())
		}
	}
	return ex.AbstractAction.SanityCheck()
}

// ToEthTx converts action to eth-compatible tx
func (ex *Execution) ToEthTx() (*types.Transaction, error) {
	if ex.contract == EmptyAddress {
		return types.NewContractCreation(ex.Nonce(), ex.amount, ex.GasLimit(), ex.GasPrice(), ex.data), nil
	}
	addr, err := address.FromString(ex.contract)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	return types.NewTransaction(ex.Nonce(), ethAddr, ex.amount, ex.GasLimit(), ex.GasPrice(), ex.data), nil
}
