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
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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
	reward_common
	amount  *big.Int
	address address.Address
	data    []byte
}

func NewClaimFromRewardingFund(amount *big.Int,
	address address.Address,
	data []byte) *ClaimFromRewardingFund {
	return &ClaimFromRewardingFund{
		amount:  amount,
		address: address,
		data:    data,
	}
}

// ClaimAmount returns the amount to claim
// note that this amount won't be charged/deducted from sender
func (c *ClaimFromRewardingFund) ClaimAmount() *big.Int { return c.amount }

// Address returns the the account to claim
// if it's nil, the default is the action sender
func (c *ClaimFromRewardingFund) Address() address.Address { return c.address }

// Data returns the additional data
func (c *ClaimFromRewardingFund) Data() []byte { return c.data }

// Serialize returns a raw byte stream of a claim action
func (c *ClaimFromRewardingFund) Serialize() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

func (act *ClaimFromRewardingFund) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_ClaimFromRewardingFund{ClaimFromRewardingFund: act.Proto()}
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

// SanityCheck validates the variables in the action
func (c *ClaimFromRewardingFund) SanityCheck() error {
	if c.ClaimAmount().Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}

// EthData returns the ABI-encoded data for converting to eth tx
func (c *ClaimFromRewardingFund) EthData() ([]byte, error) {
	if c.address == nil {
		// this is v1 ABI before adding address field
		data, err := _claimRewardingMethodV1.Inputs.Pack(c.ClaimAmount(), c.Data())
		if err != nil {
			return nil, err
		}
		return append(_claimRewardingMethodV1.ID, data...), nil
	}
	data, err := _claimRewardingMethodV2.Inputs.Pack(c.ClaimAmount(), c.address.String(), c.Data())
	if err != nil {
		return nil, err
	}
	return append(_claimRewardingMethodV2.ID, data...), nil
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
