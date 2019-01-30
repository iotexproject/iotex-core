// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"math"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	claimFromProducerFundBaseGas    = uint64(10000)
	claimFromProducerFundGasPerByte = uint64(100)
)

type ClaimFromProducerFund struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
}

// Amount returns the amount to claim
func (c *ClaimFromProducerFund) Amount() *big.Int { return c.amount }

// Data returns the additional data
func (c *ClaimFromProducerFund) Data() []byte { return c.data }

// ByteStream returns a raw byte stream of a claim action
func (c *ClaimFromProducerFund) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(c.Proto()))
}

// Proto converts a claim action struct to a claim action protobuf
func (c *ClaimFromProducerFund) Proto() *iproto.ClaimFromProducerFund {
	return &iproto.ClaimFromProducerFund{
		Amount: c.amount.Bytes(),
		Data:   c.data,
	}
}

// LoadProto converts a claim action protobuf to a claim action struct
func (c *ClaimFromProducerFund) LoadProto(claim *iproto.ClaimFromProducerFund) error {
	*c = ClaimFromProducerFund{}
	c.amount = big.NewInt(0).SetBytes(claim.Amount)
	c.data = claim.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a claim action
func (c *ClaimFromProducerFund) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(c.Data()))
	if (math.MaxUint64-claimFromProducerFundBaseGas)/claimFromProducerFundGasPerByte < dataLen {
		return 0, action.ErrOutOfGas
	}
	return claimFromProducerFundBaseGas + claimFromProducerFundGasPerByte*dataLen, nil
}

// Cost returns the total cost of a claim action
func (c *ClaimFromProducerFund) Cost() (*big.Int, error) {
	intrinsicGas, err := c.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the claim action")
	}
	return big.NewInt(0).Mul(c.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// ClaimFromProducerFundBuilder is the struct to build ClaimFromProducerFund
type ClaimFromProducerFundBuilder struct {
	action.Builder
	claim ClaimFromProducerFund
}

// SetAmount sets the amount to claim
func (b *ClaimFromProducerFundBuilder) SetAmount(amount *big.Int) *ClaimFromProducerFundBuilder {
	b.claim.amount = amount
	return b
}

// SetData sets the additional data
func (b *ClaimFromProducerFundBuilder) SetData(data []byte) *ClaimFromProducerFundBuilder {
	b.claim.data = data
	return b
}

// Build builds a new claim from producer fund action
func (b *ClaimFromProducerFundBuilder) Build() ClaimFromProducerFund {
	b.claim.AbstractAction = b.Builder.Build()
	return b.claim
}
