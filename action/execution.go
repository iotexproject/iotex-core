// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// const
const (
	EmptyAddress                     = ""
	ExecutionDataGas          uint64 = 100   // per-byte execution data gas
	ExecutionBaseIntrinsicGas uint64 = 10000 // base intrinsic gas for execution
	TxAccessListAddressGas    uint64 = 2400  // Per address specified in EIP 2930 access list
	TxAccessListStorageKeyGas uint64 = 1900  // Per storage key specified in EIP 2930 access list
)

var (
	_ hasDestination      = (*Execution)(nil)
	_ hasSize             = (*Execution)(nil)
	_ EthCompatibleAction = (*Execution)(nil)
	_ amountForCost       = (*Execution)(nil)
	_ gasLimitForCost     = (*Execution)(nil)
)

// Execution defines the struct of account-based contract execution
type Execution struct {
	contract string
	amount   *big.Int
	data     []byte
}

// NewExecution returns an Execution instance (w/o access list)
func NewExecution(contract string, amount *big.Int, data []byte) *Execution {
	return &Execution{
		contract: contract,
		amount:   amount,
		data:     data,
	}
}

// To returns the contract address pointer
// nil indicates a contract-creation transaction
func (ex *Execution) To() *common.Address {
	if ex.contract == EmptyAddress {
		return nil
	}
	addr, err := address.FromString(ex.contract)
	if err != nil {
		panic(err)
	}
	evmAddr := common.BytesToAddress(addr.Bytes())
	return &evmAddr
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

// Size returns the size of this Execution
func (ex *Execution) Size() uint32 {
	var size uint32
	if ex.amount != nil && len(ex.amount.Bytes()) > 0 {
		size += uint32(len(ex.amount.Bytes()))
	}
	// 65 is the pubkey size
	return size + uint32(len(ex.data)) + 65
}

// Serialize returns a raw byte stream of this Transfer
func (ex *Execution) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ex.Proto()))
}

func (act *Execution) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_Execution{Execution: act.Proto()}
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
	return nil
}

// IntrinsicGas returns the intrinsic gas of an execution
func (ex *Execution) IntrinsicGas() (uint64, error) {
	gas, err := CalculateIntrinsicGas(ExecutionBaseIntrinsicGas, ExecutionDataGas, uint64(len(ex.Data())))
	if err != nil {
		return gas, err
	}
	return gas, nil
}

// GasLimitForCost is an empty func to indicate that gas limit should be used
// to calculate action's cost
func (ex *Execution) GasLimitForCost() {}

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
	return nil
}

// EthTo returns the address for converting to eth tx
func (ex *Execution) EthTo() (*common.Address, error) {
	if ex.contract == EmptyAddress {
		return nil, nil
	}
	addr, err := address.FromString(ex.contract)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	return &ethAddr, nil
}

// Value returns the value for converting to eth tx
func (ex *Execution) Value() *big.Int {
	return ex.amount
}

// EthData returns the data for converting to eth tx
func (ex *Execution) EthData() ([]byte, error) {
	return ex.data, nil
}
