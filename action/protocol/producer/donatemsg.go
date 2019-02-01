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
	donateToProducerFundBaseGas    = uint64(10000)
	donateToProducerFundGasPerByte = uint64(100)
)

// DonateToProducerFund is the action to donate to the block producer fund
type DonateToProducerFund struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
}

// Amount returns the amount to donate
func (d *DonateToProducerFund) Amount() *big.Int { return d.amount }

// Data returns the additional data
func (d *DonateToProducerFund) Data() []byte { return d.data }

// ByteStream returns a raw byte stream of a donate action
func (d *DonateToProducerFund) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(d.Proto()))
}

// Proto converts a donate action struct to a donate action protobuf
func (d *DonateToProducerFund) Proto() *iproto.DonateToProducerFund {
	return &iproto.DonateToProducerFund{
		Amount: d.amount.Bytes(),
		Data:   d.data,
	}
}

// LoadProto converts a donate action protobuf to a donate action struct
func (d *DonateToProducerFund) LoadProto(donate *iproto.DonateToProducerFund) error {
	*d = DonateToProducerFund{}
	d.amount = big.NewInt(0).SetBytes(donate.Amount)
	d.data = donate.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a donate action
func (d *DonateToProducerFund) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(d.Data()))
	if (math.MaxUint64-donateToProducerFundBaseGas)/donateToProducerFundBaseGas < dataLen {
		return 0, action.ErrOutOfGas
	}
	return donateToProducerFundBaseGas + donateToProducerFundBaseGas*dataLen, nil
}

// Cost returns the total cost of a donate action
func (d *DonateToProducerFund) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the donate action")
	}
	return big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// DonateToProducerFundBuilder is the struct to build DonateToProducerFund
type DonateToProducerFundBuilder struct {
	action.Builder
	donate DonateToProducerFund
}

// SetAmount sets the amount to donate
func (b *DonateToProducerFundBuilder) SetAmount(amount *big.Int) *DonateToProducerFundBuilder {
	b.donate.amount = amount
	return b
}

// SetData sets the additional data
func (b *DonateToProducerFundBuilder) SetData(data []byte) *DonateToProducerFundBuilder {
	b.donate.data = data
	return b
}

// Build builds a new donate to producer fund action
func (b *DonateToProducerFundBuilder) Build() DonateToProducerFund {
	b.donate.AbstractAction = b.Builder.Build()
	return b.donate
}
