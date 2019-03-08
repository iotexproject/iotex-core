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
	depositToRewardingFundBaseGas = uint64(10000)
)

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

// ByteStream returns a raw byte stream of a deposit action
func (d *DepositToRewardingFund) ByteStream() []byte {
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
	amount, ok := big.NewInt(0).SetString(deposit.Amount, 10)
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
	if (math.MaxUint64-depositToRewardingFundBaseGas)/depositToRewardingFundBaseGas < dataLen {
		return 0, ErrOutOfGas
	}
	return depositToRewardingFundBaseGas + depositToRewardingFundBaseGas*dataLen, nil
}

// Cost returns the total cost of a deposit action
func (d *DepositToRewardingFund) Cost() (*big.Int, error) {
	intrinsicGas, err := d.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the deposit action")
	}
	return big.NewInt(0).Mul(d.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// DonateToRewardingFundBuilder is the struct to build DepositToRewardingFund
type DonateToRewardingFundBuilder struct {
	Builder
	deposit DepositToRewardingFund
}

// SetAmount sets the amount to deposit
func (b *DonateToRewardingFundBuilder) SetAmount(amount *big.Int) *DonateToRewardingFundBuilder {
	b.deposit.amount = amount
	return b
}

// SetData sets the additional data
func (b *DonateToRewardingFundBuilder) SetData(data []byte) *DonateToRewardingFundBuilder {
	b.deposit.data = data
	return b
}

// Build builds a new deposit to rewarding fund action
func (b *DonateToRewardingFundBuilder) Build() DepositToRewardingFund {
	b.deposit.AbstractAction = b.Builder.Build()
	return b.deposit
}
