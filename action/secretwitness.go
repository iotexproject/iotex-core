// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// SecretWitness defines the struct of DKG secret witness
type SecretWitness struct {
	AbstractAction
	witness [][]byte
}

// NewSecretWitness returns a SecretWitness instance
func NewSecretWitness(
	nonce uint64,
	sender string,
	witness [][]byte,
) (*SecretWitness, error) {
	if len(sender) == 0 {
		return nil, errors.Wrap(ErrAddress, "address of sender is empty")
	}
	return &SecretWitness{
		AbstractAction: AbstractAction{
			version: version.ProtocolVersion,
			nonce:   nonce,
			srcAddr: sender,
		},
		witness: witness,
	}, nil
}

// Witness returns the witness
func (sw *SecretWitness) Witness() [][]byte { return sw.witness }

// ByteStream returns a raw byte stream of this SecretWitness
func (sw *SecretWitness) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(sw.Proto()))
}

// Proto converts SecretWitness to protobuf's ActionPb
func (sw *SecretWitness) Proto() *iproto.SecretWitnessPb {
	// used by account-based model
	return &iproto.SecretWitnessPb{
		Witness: sw.witness,
	}
}

// LoadProto converts a protobuf's ActionPb to SecretWitness
func (sw *SecretWitness) LoadProto(pbSw *iproto.SecretWitnessPb) error {
	if pbSw == nil {
		return errors.New("empty action proto to load")
	}
	if sw == nil {
		return errors.New("nil action to load proto")
	}
	*sw = SecretWitness{}
	sw.witness = pbSw.Witness
	return nil
}

// IntrinsicGas returns the intrinsic gas of a secret witness
func (sw *SecretWitness) IntrinsicGas() (uint64, error) { return 0, nil }
