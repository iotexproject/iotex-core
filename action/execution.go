// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// EmptyAddress is the empty string
	EmptyAddress = ""
	// ExecutionDataGas represents the execution data gas per uint
	ExecutionDataGas = uint64(100)
	// ExecutionBaseIntrinsicGas represents the base intrinsic gas for execution
	ExecutionBaseIntrinsicGas = uint64(10000)
)

var _ hasDestination = (*Execution)(nil)

// Execution defines the struct of account-based contract execution
type Execution struct {
	AbstractAction

	contract string
	amount   *big.Int
	data     []byte
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
	return act
}

// LoadProto converts a protobuf's Execution to Execution
func (ex *Execution) LoadProto(pbAct *iotextypes.Execution) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if ex == nil {
		return errors.New("nil action to load proto")
	}
	*ex = Execution{}

	ex.contract = pbAct.GetContract()
	ex.amount = &big.Int{}
	_, ok := ex.amount.SetString(pbAct.GetAmount(), 10)
	if !ok {
		return errors.New("failed to set proto amount")
	}
	ex.data = pbAct.GetData()
	return nil
}

// IntrinsicGas returns the intrinsic gas of an execution
func (ex *Execution) IntrinsicGas() (uint64, error) {
	dataSize := uint64(len(ex.Data()))
	return CalculateIntrinsicGas(ExecutionBaseIntrinsicGas, ExecutionDataGas, dataSize)
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
