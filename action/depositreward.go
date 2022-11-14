// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const _depositRewardInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"internalType": "uint8[]",
				"name": "data",
				"type": "uint8[]"
			}
		],
		"name": "deposit",
		"outputs": [],
		"stateMutability": "payable",
		"type": "function"
	}
]`

var (
	// DepositToRewardingFundBaseGas represents the base intrinsic gas for depositToRewardingFund
	DepositToRewardingFundBaseGas = uint64(10000)
	// DepositToRewardingFundGasPerByte represents the depositToRewardingFund payload gas per uint
	DepositToRewardingFundGasPerByte = uint64(100)

	_depositRewardMethod abi.Method
)

func init() {
	rewardingInterface, err := abi.JSON(strings.NewReader(_depositRewardInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_depositRewardMethod, ok = rewardingInterface.Methods["deposit"]
	if !ok {
		panic("fail to load the deposit method")
	}
}

// DepositToRewardingFund is the action to deposit to the rewarding fund
type DepositToRewardingFund struct {
	AbstractAction

	amount *big.Int
	data   []byte
}

// Amount returns the amount to deposit
func (d *DepositToRewardingFund) Amount() *big.Int { return d.amount }

// Data returns the additional data
func (d *DepositToRewardingFund) Data() []byte { return d.data }

// Serialize returns a raw byte stream of a deposit action
func (d *DepositToRewardingFund) Serialize() []byte {
	return byteutil.Must(proto.Marshal(d.Proto()))
}

// Proto converts a deposit action struct to a deposit action protobuf
func (d *DepositToRewardingFund) Proto() *iotextypes.DepositToRewardingFund {
	return &iotextypes.DepositToRewardingFund{
		Amount: d.amount.String(),
		Data:   d.data,
	}
}

// LoadProto converts a deposit action protobuf to a deposit action struct
func (d *DepositToRewardingFund) LoadProto(deposit *iotextypes.DepositToRewardingFund) error {
	*d = DepositToRewardingFund{}
	amount, ok := new(big.Int).SetString(deposit.Amount, 10)
	if !ok {
		return errors.New("failed to set deposit amount")
	}
	d.amount = amount
	d.data = deposit.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a deposit action
func (d *DepositToRewardingFund) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(d.Data()))
	return CalculateIntrinsicGas(DepositToRewardingFundBaseGas, DepositToRewardingFundGasPerByte, dataLen)
}

// Cost returns the total cost of a deposit action
func (d *DepositToRewardingFund) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the deposit action")
	}
	fee := big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return new(big.Int).Add(d.Amount(), fee), nil
}

// SanityCheck validates the variables in the action
func (d *DepositToRewardingFund) SanityCheck() error {
	if d.Amount().Sign() < 0 {
		return ErrNegativeValue
	}

	return d.AbstractAction.SanityCheck()
}

// DepositToRewardingFundBuilder is the struct to build DepositToRewardingFund
type DepositToRewardingFundBuilder struct {
	Builder
	deposit DepositToRewardingFund
}

// SetAmount sets the amount to deposit
func (b *DepositToRewardingFundBuilder) SetAmount(amount *big.Int) *DepositToRewardingFundBuilder {
	b.deposit.amount = amount
	return b
}

// SetData sets the additional data
func (b *DepositToRewardingFundBuilder) SetData(data []byte) *DepositToRewardingFundBuilder {
	b.deposit.data = data
	return b
}

// Build builds a new deposit to rewarding fund action
func (b *DepositToRewardingFundBuilder) Build() DepositToRewardingFund {
	b.deposit.AbstractAction = b.Builder.Build()
	return b.deposit
}

// EncodeABIBinary encodes data in abi encoding
func (d *DepositToRewardingFund) EncodeABIBinary() ([]byte, error) {
	data, err := _depositRewardMethod.Inputs.Pack(d.Amount(), d.Data())
	if err != nil {
		return nil, err
	}
	return append(_depositRewardMethod.ID, data...), nil
}

// ToEthTx converts action to eth-compatible tx
func (d *DepositToRewardingFund) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.RewardingProtocol)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := d.EncodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(d.Nonce(), ethAddr, d.Amount(), d.GasLimit(), d.GasPrice(), data), nil
}

// NewDepositToRewardingFundFromABIBinary decodes data into action
func NewDepositToRewardingFundFromABIBinary(data []byte) (*DepositToRewardingFund, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		ac        DepositToRewardingFund
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_depositRewardMethod.ID[:], data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _depositRewardMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if ac.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if ac.data, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &ac, nil
}
