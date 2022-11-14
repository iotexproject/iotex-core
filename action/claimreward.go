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
	}
]`

var (
	// ClaimFromRewardingFundBaseGas represents the base intrinsic gas for claimFromRewardingFund
	ClaimFromRewardingFundBaseGas = uint64(10000)
	// ClaimFromRewardingFundGasPerByte represents the claimFromRewardingFund payload gas per uint
	ClaimFromRewardingFundGasPerByte = uint64(100)

	_claimRewardingMethod abi.Method
)

func init() {
	rewardingInterface, err := abi.JSON(strings.NewReader(_claimRewardingInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_claimRewardingMethod, ok = rewardingInterface.Methods["claim"]
	if !ok {
		panic("fail to load the claim method")
	}
}

// ClaimFromRewardingFund is the action to claim reward from the rewarding fund
type ClaimFromRewardingFund struct {
	AbstractAction

	amount *big.Int
	data   []byte
}

// Amount returns the amount to claim
func (c *ClaimFromRewardingFund) Amount() *big.Int { return c.amount }

// Data returns the additional data
func (c *ClaimFromRewardingFund) Data() []byte { return c.data }

// Serialize returns a raw byte stream of a claim action
func (c *ClaimFromRewardingFund) Serialize() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

// Proto converts a claim action struct to a claim action protobuf
func (c *ClaimFromRewardingFund) Proto() *iotextypes.ClaimFromRewardingFund {
	return &iotextypes.ClaimFromRewardingFund{
		Amount: c.amount.String(),
		Data:   c.data,
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

// SetData sets the additional data
func (b *ClaimFromRewardingFundBuilder) SetData(data []byte) *ClaimFromRewardingFundBuilder {
	b.claim.data = data
	return b
}

// Build builds a new claim from rewarding fund action
func (b *ClaimFromRewardingFundBuilder) Build() ClaimFromRewardingFund {
	b.claim.AbstractAction = b.Builder.Build()
	return b.claim
}

// EncodeABIBinary encodes data in abi encoding
func (c *ClaimFromRewardingFund) EncodeABIBinary() ([]byte, error) {
	data, err := _claimRewardingMethod.Inputs.Pack(c.Amount(), c.Data())
	if err != nil {
		return nil, err
	}
	return append(_claimRewardingMethod.ID, data...), nil
}

// ToEthTx converts action to eth-compatible tx
func (c *ClaimFromRewardingFund) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.RewardingProtocol)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := c.EncodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(c.Nonce(), ethAddr, big.NewInt(0), c.GasLimit(), c.GasPrice(), data), nil
}

// NewClaimFromRewardingFundFromABIBinary decodes data into action
func NewClaimFromRewardingFundFromABIBinary(data []byte) (*ClaimFromRewardingFund, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		ac        ClaimFromRewardingFund
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_claimRewardingMethod.ID[:], data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _claimRewardingMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
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
