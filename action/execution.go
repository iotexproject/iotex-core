// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	iproto "github.com/iotexproject/iotex-core/proto"
)

const (
	// EmptyAddress is the empty string
	EmptyAddress = ""
	// ExecutionDataGas represents the execution data gas per uint
	ExecutionDataGas = uint64(100)
	// ExecutionBaseIntrinsicGas represents the base intrinsic gas for execution
	ExecutionBaseIntrinsicGas = uint64(10000)
)

// Execution defines the struct of account-based contract execution
type Execution struct {
	AbstractAction
	amount *big.Int
	data   []byte
}

// NewExecution returns a Execution instance
func NewExecution(executorAddress string, contractAddress string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (*Execution, error) {
	if executorAddress == "" {
		return nil, errors.Wrap(ErrAddress, "address of the executor is empty")
	}

	return &Execution{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  executorAddress,
			dstAddr:  contractAddress,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		amount: amount,
		data:   data,
	}, nil
}

// Executor returns an executor address
func (ex *Execution) Executor() string {
	return ex.SrcAddr()
}

// ExecutorPublicKey returns the executor's public key
func (ex *Execution) ExecutorPublicKey() keypair.PublicKey {
	return ex.SrcPubkey()
}

// Contract returns a contract address
func (ex *Execution) Contract() string {
	return ex.DstAddr()
}

// Amount returns the amount
func (ex *Execution) Amount() *big.Int {
	return ex.amount
}

// Data returns the data bytes
func (ex *Execution) Data() []byte {
	return ex.data
}

// TotalSize returns the total size of this Execution
func (ex *Execution) TotalSize() uint32 {
	size := ex.BasicActionSize()
	if ex.amount != nil && len(ex.amount.Bytes()) > 0 {
		size += uint32(len(ex.amount.Bytes()))
	}

	return size + uint32(len(ex.data))
}

// ByteStream returns a raw byte stream of this Transfer
func (ex *Execution) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(ex.Proto()))
}

// Proto converts Execution to protobuf's ExecutionPb
func (ex *Execution) Proto() *iproto.ExecutionPb {
	act := &iproto.ExecutionPb{
		Contract: ex.dstAddr,
		Data:     ex.data,
	}
	if ex.amount != nil && len(ex.amount.Bytes()) > 0 {
		act.Amount = ex.amount.Bytes()
	}
	return act
}

// LoadProto converts a protobuf's ExecutionPb to Execution
func (ex *Execution) LoadProto(pbAct *iproto.ExecutionPb) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if ex == nil {
		return errors.New("nil action to load proto")
	}
	*ex = Execution{}

	ex.data = pbAct.GetData()
	ex.amount = &big.Int{}
	ex.amount.SetBytes(pbAct.GetAmount())
	return nil
}

// IntrinsicGas returns the intrinsic gas of an execution
func (ex *Execution) IntrinsicGas() (uint64, error) {
	dataSize := uint64(len(ex.Data()))
	if (math.MaxUint64-ExecutionBaseIntrinsicGas)/ExecutionDataGas < dataSize {
		return 0, ErrOutOfGas
	}

	return dataSize*ExecutionDataGas + ExecutionBaseIntrinsicGas, nil
}

// Cost returns the cost of an execution
func (ex *Execution) Cost() (*big.Int, error) {
	maxExecFee := big.NewInt(0).Mul(ex.GasPrice(), big.NewInt(0).SetUint64(ex.GasLimit()))
	return big.NewInt(0).Add(ex.Amount(), maxExecFee), nil
}
