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
	// UpdateActiveProtocolsBaseGas represents the base intrinsic gas for UpdateActiveProtocols
	UpdateActiveProtocolsBaseGas = uint64(10000)
	// UpdateActiveProtocolsPerByte represents the UpdateActiveProtocols payload gas per uint
	UpdateActiveProtocolsPerByte = uint64(100)
)

// UpdateActiveProtocols is the action the update active protocols
type UpdateActiveProtocols struct {
	AbstractAction

	additions []string
	removals  []string
	data      []byte
}

// Additions returns the additions
func (uap *UpdateActiveProtocols) Additions() []string { return uap.additions }

// Removals returns the removals
func (uap *UpdateActiveProtocols) Removals() []string { return uap.removals }

// Data returns the additional data
func (uap *UpdateActiveProtocols) Data() []byte { return uap.data }

// ByteStream returns a raw byte stream of an UpdateActiveProtocols action
func (uap *UpdateActiveProtocols) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(uap.Proto()))
}

// Proto converts an UpdateActiveProtocols action struct to an UpdateActiveProtocols action protobuf
func (uap *UpdateActiveProtocols) Proto() *iotextypes.UpdateActiveProtocols {
	return &iotextypes.UpdateActiveProtocols{
		Additions: uap.additions,
		Removals:  uap.removals,
		Data:      uap.data,
	}
}

// LoadProto converts an UpdateActiveProtocols action protobuf to an UpdateActiveProtocols action struct
func (uap *UpdateActiveProtocols) LoadProto(update *iotextypes.UpdateActiveProtocols) error {
	*uap = UpdateActiveProtocols{
		additions: update.Additions,
		removals:  update.Removals,
		data:      update.Data,
	}
	return nil
}

// IntrinsicGas returns the intrinsic gas of an UpdateActiveProtocols action
func (uap *UpdateActiveProtocols) IntrinsicGas() (uint64, error) {
	dataLen := len(uap.Data())
	for _, pid := range uap.additions {
		dataLen += len(pid)
	}
	for _, pid := range uap.removals {
		dataLen += len(pid)
	}
	if (math.MaxUint64-UpdateActiveProtocolsBaseGas)/UpdateActiveProtocolsPerByte < uint64(dataLen) {
		return 0, ErrOutOfGas
	}
	return UpdateActiveProtocolsBaseGas + UpdateActiveProtocolsPerByte*uint64(dataLen), nil
}

// Cost returns the total cost of an UpdateActiveProtocols action
func (uap *UpdateActiveProtocols) Cost() (*big.Int, error) {
	intrinsicGas, err := uap.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "error when getting intrinsic gas for the UpdateActiveProtocols action")
	}
	return big.NewInt(0).Mul(uap.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas)), nil
}

// UpdateActiveProtocolsBuilder is the builder to build UpdateActiveProtocols
type UpdateActiveProtocolsBuilder struct {
	Builder
	uap UpdateActiveProtocols
}

// SetAdditions sets additions
func (b *UpdateActiveProtocolsBuilder) SetAdditions(additions []string) *UpdateActiveProtocolsBuilder {
	b.uap.additions = additions
	return b
}

// SetRemovals sets removals
func (b *UpdateActiveProtocolsBuilder) SetRemovals(removals []string) *UpdateActiveProtocolsBuilder {
	b.uap.removals = removals
	return b
}

// SetData sets the additional data
func (b *UpdateActiveProtocolsBuilder) SetData(data []byte) *UpdateActiveProtocolsBuilder {
	b.uap.data = data
	return b
}

// Build builds a new UpdateActiveProtocols action
func (b *UpdateActiveProtocolsBuilder) Build() UpdateActiveProtocols {
	b.uap.AbstractAction = b.Builder.Build()
	return b.uap
}
