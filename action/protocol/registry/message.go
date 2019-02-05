// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package registry

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// RegisterProtocol is the action to register protocol
type RegisterProtocol struct {
	action.AbstractAction
	code []byte
}

// ByteStream returns a raw byte stream of this instance
func (r *RegisterProtocol) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(r.Proto()))
}

// Proto converts a RegisterProtocol struct toa  protobuf's RegisterProtocol
func (r *RegisterProtocol) Proto() *iproto.RegisterProtocol {
	return &iproto.RegisterProtocol{
		Code: r.code,
	}
}

// LoadProto converts a protobuf's RegisterProtocol to a RegisterProtocol struct
func (r *RegisterProtocol) LoadProto(proto *iproto.RegisterProtocol) error {
	*r = RegisterProtocol{}
	r.code = proto.GetCode()
	return nil
}

// IntrinsicGas returns the intrinsic gas of a RegisterProtocol. Make it free for installing a protocol
func (r *RegisterProtocol) IntrinsicGas() (uint64, error) { return 0, nil }

// Cost returns the cost of a RegisterProtocol
func (r *RegisterProtocol) Cost() (*big.Int, error) { return big.NewInt(0), nil }

// Code returns the code of protocol
func (r *RegisterProtocol) Code() []byte { return r.code }

// RegisterProtocolBuilder is the builder to build RegisterProtocol
type RegisterProtocolBuilder struct {
	action.Builder
	registerProtocol RegisterProtocol
}

// SetCode sets the code of protocol
func (b *RegisterProtocolBuilder) SetCode(code []byte) *RegisterProtocolBuilder {
	b.registerProtocol.code = code
	return b
}

// Build builds RegisterProtocol
func (b *RegisterProtocolBuilder) Build() RegisterProtocol {
	b.registerProtocol.AbstractAction = b.Builder.Build()
	return b.registerProtocol
}
