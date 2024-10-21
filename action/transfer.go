// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// TransferPayloadGas represents the transfer payload gas per uint
	TransferPayloadGas = uint64(100)
	// TransferBaseIntrinsicGas represents the base intrinsic gas for transfer
	TransferBaseIntrinsicGas = uint64(10000)
)

var (
	_ hasDestination      = (*Transfer)(nil)
	_ hasSize             = (*Transfer)(nil)
	_ EthCompatibleAction = (*Transfer)(nil)
	_ amountForCost       = (*Transfer)(nil)
)

// Transfer defines the struct of account-based transfer
type Transfer struct {
	amount    *big.Int
	recipient string
	payload   []byte
}

// NewTransfer returns a Transfer instance
func NewTransfer(
	amount *big.Int,
	recipient string,
	payload []byte,
) *Transfer {
	return &Transfer{
		recipient: recipient,
		amount:    amount,
		payload:   payload,
	}
}

// Amount returns the amount
func (tsf *Transfer) Amount() *big.Int { return tsf.amount }

// Payload returns the payload bytes
func (tsf *Transfer) Payload() []byte { return tsf.payload }

// Recipient returns the recipient address. It's the wrapper of Action.DstAddr
func (tsf *Transfer) Recipient() string { return tsf.recipient }

// Destination returns the recipient address as destination.
func (tsf *Transfer) Destination() string { return tsf.recipient }

// Size returns the total size of this Transfer
func (tsf *Transfer) Size() uint32 {
	var size uint32
	if tsf.amount != nil && len(tsf.amount.Bytes()) > 0 {
		size += uint32(len(tsf.amount.Bytes()))
	}
	// 65 is the pubkey size
	return size + uint32(len(tsf.payload)) + 65
}

// Serialize returns a raw byte stream of this Transfer
func (tsf *Transfer) Serialize() []byte {
	return byteutil.Must(proto.Marshal(tsf.Proto()))
}

func (tsf *Transfer) FillAction(act *iotextypes.ActionCore) {
	act.Action = &iotextypes.ActionCore_Transfer{Transfer: tsf.Proto()}
}

// Proto converts Transfer to protobuf's Action
func (tsf *Transfer) Proto() *iotextypes.Transfer {
	// used by account-based model
	act := &iotextypes.Transfer{
		Recipient: tsf.recipient,
		Payload:   tsf.payload,
	}

	if tsf.amount != nil {
		act.Amount = tsf.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to Transfer
func (tsf *Transfer) LoadProto(pbAct *iotextypes.Transfer) error {
	if pbAct == nil {
		return ErrNilProto
	}
	if tsf == nil {
		return ErrNilAction
	}
	*tsf = Transfer{}

	tsf.recipient = pbAct.GetRecipient()
	tsf.payload = pbAct.GetPayload()
	if pbAct.GetAmount() == "" {
		tsf.amount = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(pbAct.GetAmount(), 10)
		// tsf amount gets zero when pbAct.GetAmount is empty string
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetAmount())
		}
		tsf.amount = amount
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of a transfer
func (tsf *Transfer) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(tsf.Payload()))
	return CalculateIntrinsicGas(TransferBaseIntrinsicGas, TransferPayloadGas, payloadSize)
}

// SanityCheck validates the variables in the action
func (tsf *Transfer) SanityCheck() error {
	// Reject transfer of negative amount
	if tsf.Amount().Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}

// EthTo returns the address for converting to eth tx
func (tsf *Transfer) EthTo() (*common.Address, error) {
	addr, err := address.FromString(tsf.recipient)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	return &ethAddr, nil
}

// Value returns the value for converting to eth tx
func (tsf *Transfer) Value() *big.Int {
	return tsf.amount
}

// EthData returns the data for converting to eth tx
func (tsf *Transfer) EthData() ([]byte, error) {
	return tsf.payload, nil
}
