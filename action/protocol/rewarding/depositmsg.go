// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

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
	depositToRewardingFundBaseGas    = uint64(10000)
	depositToRewardingFundGasPerByte = uint64(100)
)

// DonateToRewardingFund is the action to donate to the rewarding fund
type DonateToRewardingFund struct {
	action.AbstractAction
	amount *big.Int
	data   []byte
}

// Amount returns the amount to donate
func (d *DonateToRewardingFund) Amount() *big.Int { return d.amount }

// Data returns the additional data
func (d *DonateToRewardingFund) Data() []byte { return d.data }

// ByteStream returns a raw byte stream of a donate action
func (d *DonateToRewardingFund) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(d.Proto()))
}

// Proto converts a donate action struct to a donate action protobuf
func (d *DonateToRewardingFund) Proto() *iproto.DepositToRewardingFund {
	return &iproto.DepositToRewardingFund{
		Amount: d.amount.Bytes(),
		Data:   d.data,
	}
}

// LoadProto converts a donate action protobuf to a donate action struct
func (d *DonateToRewardingFund) LoadProto(donate *iproto.DepositToRewardingFund) error {
	*d = DonateToRewardingFund{}
	d.amount = big.NewInt(0).SetBytes(donate.Amount)
	d.data = donate.Data
	return nil
}

// IntrinsicGas returns the intrinsic gas of a donate action
func (d *DonateToRewardingFund) IntrinsicGas() (uint64, error) {
	dataLen := uint64(len(d.Data()))
	if (math.MaxUint64-depositToRewardingFundBaseGas)/depositToRewardingFundBaseGas < dataLen {
		return 0, action.ErrOutOfGas
	}
	return depositToRewardingFundBaseGas + depositToRewardingFundBaseGas*dataLen, nil
}

// Cost returns the total cost of a donate action
func (d *DonateToRewardingFund) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the donate action")
	}
	return big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// DonateToRewardingFundBuilder is the struct to build DonateToRewardingFund
type DonateToRewardingFundBuilder struct {
	action.Builder
	donate DonateToRewardingFund
}

// SetAmount sets the amount to donate
func (b *DonateToRewardingFundBuilder) SetAmount(amount *big.Int) *DonateToRewardingFundBuilder {
	b.donate.amount = amount
	return b
}

// SetData sets the additional data
func (b *DonateToRewardingFundBuilder) SetData(data []byte) *DonateToRewardingFundBuilder {
	b.donate.data = data
	return b
}

// Build builds a new donate to rewarding fund action
func (b *DonateToRewardingFundBuilder) Build() DonateToRewardingFund {
	b.donate.AbstractAction = b.Builder.Build()
	return b.donate
}
