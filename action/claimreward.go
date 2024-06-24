// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const _claimRewardingInterfaceABI = `[
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
		"name": "claim",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"internalType": "string",
				"name": "address",
				"type": "string"
			},
			{
				"internalType": "uint8[]",
				"name": "data",
				"type": "uint8[]"
			}
		],
		"name": "claimFor",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

var (
	// ClaimFromRewardingFundBaseGas represents the base intrinsic gas for claimFromRewardingFund
	ClaimFromRewardingFundBaseGas = uint64(10000)
	// ClaimFromRewardingFundGasPerByte represents the claimFromRewardingFund payload gas per uint
	ClaimFromRewardingFundGasPerByte = uint64(100)

	_claimRewardingMethodV1 abi.Method
	_claimRewardingMethodV2 abi.Method
	_                       EthCompatibleAction = (*ClaimFromRewardingFund)(nil)
	errWrongMethodSig                           = errors.New("wrong method signature")
)

func init() {
	claimRewardInterface, err := abi.JSON(strings.NewReader(_claimRewardingInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_claimRewardingMethodV1, ok = claimRewardInterface.Methods["claim"]
	if !ok {
		panic("fail to load the claim method")
	}
	_claimRewardingMethodV2, ok = claimRewardInterface.Methods["claimFor"]
	if !ok {
		panic("fail to load the claimTo method")
	}
}

// ClaimFromRewardingFund is the action to claim reward from the rewarding fund
type ClaimFromRewardingFund struct {
	AbstractAction

	amount  *big.Int
	address address.Address
	data    []byte
}

// Amount returns the amount to claim
func (c *ClaimFromRewardingFund) Amount() *big.Int { return c.amount }

// Address returns the the account to claim
// if it's nil, the default is the action sender
func (c *ClaimFromRewardingFund) Address() address.Address { return c.address }

// Data returns the additional data
func (c *ClaimFromRewardingFund) Data() []byte { return c.data }

// Serialize returns a raw byte stream of a claim action
func (c *ClaimFromRewardingFund) Serialize() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

// Proto converts a claim action struct to a claim action protobuf
func (c *ClaimFromRewardingFund) Proto() *iotextypes.ClaimFromRewardingFund {
	var addr string
	if c.address != nil {
		addr = c.address.String()
	}
	return &iotextypes.ClaimFromRewardingFund{
		Amount:  c.amount.String(),
		Address: addr,
		Data:    c.data,
	}
}

// LoadProto converts a claim action protobuf to a claim action struct
func (c *ClaimFromRewardingFund) LoadProto(claim *iotextypes.ClaimFromRewardingFund) error {
	*c = ClaimFromRewardingFund{}
	amount, ok := new(big.Int).SetString(claim.Amount, 10)
	if !ok {
		return errors.New("failed to set claim amount")
	}
	c.amount = amount
	c.data = claim.Data
	if len(claim.Address) > 0 {
		addr, err := address.FromString(claim.Address)
		if err != nil {
			return err
		}
		c.address = addr
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a claim action
func (c *ClaimFromRewardingFund) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(c.Data()))
	return CalculateIntrinsicGas(ClaimFromRewardingFundBaseGas, ClaimFromRewardingFundGasPerByte, dataLen)
}

// Cost returns the total cost of a claim action
func (c *ClaimFromRewardingFund) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the claim action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// SanityCheck validates the variables in the action
func (c *ClaimFromRewardingFund) SanityCheck() error {
	if c.Amount().Sign() < 0 {
		return ErrNegativeValue
	}
	return c.AbstractAction.SanityCheck()
}

// ClaimFromRewardingFundBuilder is the struct to build ClaimFromRewardingFund
type ClaimFromRewardingFundBuilder struct {
	Builder
	claim ClaimFromRewardingFund
}

// SetAmount sets the amount to claim
func (b *ClaimFromRewardingFundBuilder) SetAmount(amount *big.Int) *ClaimFromRewardingFundBuilder {
	b.claim.amount = amount
	return b
}

// SetAddress set the address to claim
func (b *ClaimFromRewardingFundBuilder) SetAddress(addr address.Address) *ClaimFromRewardingFundBuilder {
	b.claim.address = addr
	return b
}

// SetData sets the additional data
func (b *ClaimFromRewardingFundBuilder) SetData(data []byte) *ClaimFromRewardingFundBuilder {
	b.claim.data = data
	return b
}

// Reset set all field to defaults
func (b *ClaimFromRewardingFundBuilder) Reset() {
	b.Builder = Builder{}
	b.claim = ClaimFromRewardingFund{}
}

// Build builds a new claim from rewarding fund action
func (b *ClaimFromRewardingFundBuilder) Build() ClaimFromRewardingFund {
	b.claim.AbstractAction = b.Builder.Build()
	return b.claim
}

// encodeABIBinary encodes data in abi encoding
func (c *ClaimFromRewardingFund) encodeABIBinary() ([]byte, error) {
	if c.address == nil {
		// this is v1 ABI before adding address field
		data, err := _claimRewardingMethodV1.Inputs.Pack(c.Amount(), c.Data())
		if err != nil {
			return nil, err
		}
		return append(_claimRewardingMethodV1.ID, data...), nil
	}
	data, err := _claimRewardingMethodV2.Inputs.Pack(c.Amount(), c.address.String(), c.Data())
	if err != nil {
		return nil, err
	}
	return append(_claimRewardingMethodV2.ID, data...), nil
}

// ToEthTx converts action to eth-compatible tx
func (c *ClaimFromRewardingFund) ToEthTx(_ uint32) (*types.Transaction, error) {
	data, err := c.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTx(&types.LegacyTx{
		Nonce:    c.Nonce(),
		GasPrice: c.GasPrice(),
		Gas:      c.GasLimit(),
		To:       &_rewardingProtocolEthAddr,
		Value:    big.NewInt(0),
		Data:     data,
	}), nil
}

// NewClaimFromRewardingFundFromABIBinary decodes data into action
func NewClaimFromRewardingFundFromABIBinary(data []byte) (*ClaimFromRewardingFund, error) {
	if len(data) <= 4 {
		return nil, errDecodeFailure
	}

	var (
		paramsMap = map[string]interface{}{}
		ok, isV2  bool
		ac        ClaimFromRewardingFund
	)
	switch {
	case bytes.Equal(_claimRewardingMethodV2.ID[:], data[:4]):
		if err := _claimRewardingMethodV2.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
			return nil, err
		}
		isV2 = true
	case bytes.Equal(_claimRewardingMethodV1.ID[:], data[:4]):
		if err := _claimRewardingMethodV1.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
			return nil, err
		}
	default:
		return nil, errWrongMethodSig
	}

	if ac.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if ac.data, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	if isV2 {
		var s string
		if s, ok = paramsMap["address"].(string); !ok {
			return nil, errDecodeFailure
		}
		if len(s) == 0 {
			return nil, errors.Wrap(errDecodeFailure, "address is empty")
		}
		addr, err := address.FromString(s)
		if err != nil {
			return nil, err
		}
		ac.address = addr
	}
	return &ac, nil
}
