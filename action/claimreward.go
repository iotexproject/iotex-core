// Copyright (c) 2019 IoTeX
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

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var (
	claimFromRewardingFundBaseGas    = uint64(10000)
	claimFromRewardingFundGasPerByte = uint64(100)
)

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

// ByteStream returns a raw byte stream of a claim action
func (c *ClaimFromRewardingFund) ByteStream() []byte {
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
	amount, ok := big.NewInt(0).SetString(claim.Amount, 10)
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
	if (math.MaxUint64-claimFromRewardingFundBaseGas)/claimFromRewardingFundGasPerByte < dataLen {
		return 0, ErrOutOfGas
	}
	return claimFromRewardingFundBaseGas + claimFromRewardingFundGasPerByte*dataLen, nil
}

// Cost returns the total cost of a claim action
func (c *ClaimFromRewardingFund) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the claim action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
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
